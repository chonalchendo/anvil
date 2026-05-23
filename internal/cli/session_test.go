package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeSessionFixture writes a session file at 10-sessions/<filenameID>.md whose
// frontmatter session_id is fmID (usually equal to filenameID; differ them to
// exercise the collision guard) and whose body is body.
func writeSessionFixture(t *testing.T, vault, filenameID, fmID, title, body string) string {
	t.Helper()
	path := filepath.Join(vault, "10-sessions", filenameID+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "session", "session_id": fmID, "source": "claude-code",
			"status": "raw", "title": title, "created": "2026-05-23",
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSessionCurrent_JSON_ResolvesFromEnv(t *testing.T) {
	vault := setupVault(t)
	t.Setenv(envSessionID, "abc123")

	out, _, err := runCmd(t, newRootCmd(), "session", "current", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got struct {
		SessionID string `json:"session_id"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.SessionID != "abc123" {
		t.Errorf("session_id = %q, want abc123", got.SessionID)
	}
	want := filepath.Join(vault, "10-sessions", "abc123.md")
	if got.Path != want {
		t.Errorf("path = %q, want %q", got.Path, want)
	}
}

func TestSessionCurrent_UnsetEnv_Errors(t *testing.T) {
	setupVault(t)
	t.Setenv(envSessionID, "") // unset; the test process may inherit a real id

	_, _, err := runCmd(t, newRootCmd(), "session", "current")
	if err == nil {
		t.Fatal("expected error when session env var is unset")
	}
	if !strings.Contains(err.Error(), envSessionID) {
		t.Errorf("error should name %s: %q", envSessionID, err.Error())
	}
}

func TestSessionList_JSON_IsArrayNewestFirst(t *testing.T) {
	vault := setupVault(t)
	older := writeSessionFixture(t, vault, "older", "older", "Older", "")
	newer := writeSessionFixture(t, vault, "newer", "newer", "Newer",
		"## Handoff\n\n**Objective.** ship the session CLI\n")

	// Pin mtimes so the newest-first ordering is deterministic regardless of
	// filesystem timestamp granularity.
	base := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(older, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, base.Add(time.Hour), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	out, _, err := runCmd(t, newRootCmd(), "session", "list", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []sessionItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("list --json must be a plain array: %v\n%s", err, out)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %s", len(items), out)
	}
	if items[0].SessionID != "newer" {
		t.Errorf("newest-first: items[0] = %q, want newer", items[0].SessionID)
	}
	if !items[0].HasHandoff {
		t.Error("newer has a handoff body; has_handoff should be true")
	}
	if items[0].Objective != "ship the session CLI" {
		t.Errorf("objective = %q, want parsed Objective line", items[0].Objective)
	}
	if items[1].HasHandoff {
		t.Error("older is an empty stub; has_handoff should be false")
	}
}

func TestSessionList_JSON_EmptyIsArrayNotNull(t *testing.T) {
	setupVault(t)
	out, _, err := runCmd(t, newRootCmd(), "session", "list", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("empty list --json = %q, want []", strings.TrimSpace(out))
	}
}

func TestSessionHandoff_WritesBodyIntoCurrentFile(t *testing.T) {
	vault := setupVault(t)
	t.Setenv(envSessionID, "live")
	path := writeSessionFixture(t, vault, "live", "live", "Live", "")

	body := "## Handoff\n\n**Objective.** finish the work\n"
	_, _, err := runCmd(t, newRootCmd(), "session", "handoff", "--body", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Body, "**Objective.** finish the work") {
		t.Errorf("handoff body not written: %q", a.Body)
	}
	if sid, _ := a.FrontMatter["session_id"].(string); sid != "live" {
		t.Errorf("frontmatter clobbered: session_id = %q, want live", sid)
	}
}

func TestSessionHandoff_RefusesCrossSessionClobber(t *testing.T) {
	vault := setupVault(t)
	t.Setenv(envSessionID, "mine")
	// The file at mine.md stores a DIFFERENT session_id — a stale/foreign file
	// the guard must refuse to overwrite.
	writeSessionFixture(t, vault, "mine", "theirs", "Theirs", "## Handoff\n\ntheir unconsumed work\n")

	_, _, err := runCmd(t, newRootCmd(), "session", "handoff", "--body", "my handoff\n")
	if err == nil {
		t.Fatal("expected the collision guard to refuse the write")
	}
	if !strings.Contains(err.Error(), "refusing handoff") {
		t.Errorf("error should explain the refusal: %q", err.Error())
	}
}

func TestSessionHandoff_EmptyBody_Errors(t *testing.T) {
	vault := setupVault(t)
	t.Setenv(envSessionID, "live")
	writeSessionFixture(t, vault, "live", "live", "Live", "")

	_, _, err := runCmd(t, newRootCmd(), "session", "handoff", "--body", "   \n")
	if err == nil {
		t.Fatal("expected error for empty handoff body")
	}
}
