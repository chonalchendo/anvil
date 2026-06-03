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
	t.Setenv(envSessionID, "")          // unset; the test process may inherit a real id
	t.Setenv("CODEX_HOME", t.TempDir()) // empty: no Codex rollout to fall back to

	_, _, err := runCmd(t, newRootCmd(), "session", "current")
	if err == nil {
		t.Fatal("expected error when neither a Claude nor a Codex session resolves")
	}
	if !strings.Contains(err.Error(), envSessionID) {
		t.Errorf("error should name %s: %q", envSessionID, err.Error())
	}
}

// TestSessionCodexBinding covers the Codex fallback: with no Claude session id
// but a Codex rollout file present, `session current` resolves the rollout's id
// and `session handoff` lazily creates the session file (no SessionStart hook
// under Codex) and writes the handoff body resume can read back.
func TestSessionCodexBinding(t *testing.T) {
	vault := setupVault(t)
	t.Setenv(envSessionID, "")
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	const codexID = "codexsess-1111-2222-3333-444444444444"
	rollDir := filepath.Join(codexHome, "sessions", "2026", "06", "03")
	if err := os.MkdirAll(rollDir, 0o750); err != nil {
		t.Fatal(err)
	}
	roll := filepath.Join(rollDir, "rollout-2026-06-03T10-00-00-"+codexID+".jsonl")
	if err := os.WriteFile(roll, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, _, err := runCmd(t, newRootCmd(), "session", "current")
	if err != nil {
		t.Fatalf("session current under codex: %v", err)
	}
	if !strings.Contains(out, codexID) {
		t.Errorf("session current = %q, want it to resolve codex id %q", out, codexID)
	}

	if _, _, err := runCmd(t, newRootCmd(), "session", "handoff", "--body", "carry-over context", "--project", "anvil"); err != nil {
		t.Fatalf("session handoff under codex: %v", err)
	}
	sessionFile := filepath.Join(vault, "10-sessions", codexID+".md")
	a, err := core.LoadArtifact(sessionFile)
	if err != nil {
		t.Fatalf("codex handoff should have created %s: %v", sessionFile, err)
	}
	if got, _ := a.FrontMatter["source"].(string); got != "codex" {
		t.Errorf("source = %q, want codex", got)
	}
	if !strings.Contains(a.Body, "carry-over context") {
		t.Errorf("handoff body = %q, want the written context", a.Body)
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

func TestSessionShow_Body_PrintsHandoffBody(t *testing.T) {
	vault := setupVault(t)
	body := "## Handoff\n\n**Objective.** finish the work\n"
	writeSessionFixture(t, vault, "sid-abc", "sid-abc", "Test Session", body)

	out, _, err := runCmd(t, newRootCmd(), "session", "show", "sid-abc", "--body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "**Objective.** finish the work") {
		t.Errorf("body output missing expected content: %q", out)
	}
}

func TestSessionShow_JSON_IncludesBodyField(t *testing.T) {
	vault := setupVault(t)
	body := "## Handoff\n\n**Objective.** ship the feature\n"
	writeSessionFixture(t, vault, "sid-xyz", "sid-xyz", "JSON Session", body)

	out, _, err := runCmd(t, newRootCmd(), "session", "show", "sid-xyz", "--json", "--body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got sessionShowOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.SessionID != "sid-xyz" {
		t.Errorf("session_id = %q, want sid-xyz", got.SessionID)
	}
	if got.Body == nil || !strings.Contains(*got.Body, "ship the feature") {
		t.Errorf("body field missing or incorrect: %v", got.Body)
	}
}

func TestSessionShow_UnknownID_ActionableError(t *testing.T) {
	setupVault(t)
	_, stderr, err := runCmd(t, newRootCmd(), "session", "show", "not-a-real-id")
	if err == nil {
		t.Fatal("expected error for unknown session_id")
	}
	// The error must name the field "session_id" (agent-cli principle #5)
	combined := stderr + err.Error()
	if !strings.Contains(strings.ToLower(combined), "session_id") {
		t.Errorf("error should name the session_id field; got: %q", combined)
	}
}

func TestSessionResume_JSON_ReturnsSingleHandoff(t *testing.T) {
	vault := setupVault(t)
	body := "## Handoff\n\n**Objective.** continue the campaign\n"
	writeSessionFixture(t, vault, "resume-1", "resume-1", "Resume Session", body)
	// stub — no handoff
	writeSessionFixture(t, vault, "resume-2", "resume-2", "Stub Session", "")

	out, _, err := runCmd(t, newRootCmd(), "session", "resume", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got resumeOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.SessionID == "" {
		t.Error("session_id should be non-empty")
	}
	if got.Body == "" {
		t.Error("body should be non-empty")
	}
	if !strings.Contains(got.Body, "continue the campaign") {
		t.Errorf("body missing expected content: %q", got.Body)
	}
}

func TestSessionResume_NoHandoff_Errors(t *testing.T) {
	vault := setupVault(t)
	// Only stubs — no handoffs
	writeSessionFixture(t, vault, "stub-1", "stub-1", "Stub 1", "")

	_, _, err := runCmd(t, newRootCmd(), "session", "resume", "--json")
	if err == nil {
		t.Fatal("expected error when no handoff exists")
	}
}

func TestSessionResume_AmbiguityWindow_ReturnsCandidates(t *testing.T) {
	vault := setupVault(t)
	body := "## Handoff\n\n**Objective.** campaign A\n"
	body2 := "## Handoff\n\n**Objective.** campaign B\n"
	p1 := writeSessionFixture(t, vault, "amb-1", "amb-1", "Session A", body)
	p2 := writeSessionFixture(t, vault, "amb-2", "amb-2", "Session B", body2)

	// Pin both mtimes to be within 10 minutes of each other
	now := time.Now()
	if err := os.Chtimes(p1, now, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p2, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	out, _, err := runCmd(t, newRootCmd(), "session", "resume", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got resumeOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	// With two candidates in window, candidates list is returned for caller disambiguation
	if len(got.Candidates) < 2 {
		t.Errorf("expected ≥2 candidates in ambiguity window, got %d", len(got.Candidates))
	}
}

// writeSessionFixtureWithProject is like writeSessionFixture but stamps a project field.
func writeSessionFixtureWithProject(t *testing.T, vault, filenameID, fmID, title, project, body string) string {
	t.Helper()
	path := filepath.Join(vault, "10-sessions", filenameID+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "session", "session_id": fmID, "source": "claude-code",
			"status": "raw", "title": title, "created": "2026-05-23",
			"project": project,
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSessionResume_ProjectScope(t *testing.T) {
	vault := setupVault(t)
	anvilBody := "## Handoff\n\n**Objective.** anvil work\n"
	otherBody := "## Handoff\n\n**Objective.** other project work\n"

	// Two handoffs from different projects; timestamps far apart so no ambiguity.
	p1 := writeSessionFixtureWithProject(t, vault, "sess-anvil", "sess-anvil", "Anvil Session", "anvil", anvilBody)
	p2 := writeSessionFixtureWithProject(t, vault, "sess-other", "sess-other", "Other Session", "other", otherBody)
	now := time.Now()
	// anvil session is newest; other session is 20 minutes older (outside ambiguity window).
	if err := os.Chtimes(p1, now, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p2, now.Add(-20*time.Minute), now.Add(-20*time.Minute)); err != nil {
		t.Fatal(err)
	}

	// --project anvil should return only the anvil handoff.
	out, _, err := runCmd(t, newRootCmd(), "session", "resume", "--project", "anvil", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got resumeOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.SessionID != "sess-anvil" {
		t.Errorf("session_id = %q, want sess-anvil", got.SessionID)
	}
	if got.Project != "anvil" {
		t.Errorf("project = %q, want anvil", got.Project)
	}
	if !strings.Contains(got.Body, "anvil work") {
		t.Errorf("body should contain anvil content: %q", got.Body)
	}
}

func TestSessionResume_NoMatch(t *testing.T) {
	vault := setupVault(t)
	writeSessionFixtureWithProject(t, vault, "sess-anvil", "sess-anvil", "Anvil Session", "anvil",
		"## Handoff\n\n**Objective.** anvil work\n")

	// A project scope matching no handoff is exit 0 with an explicit no_handoff
	// signal — distinct from a populated hit (which always has a non-empty
	// session_id) and from the unscoped no-handoff error path.
	out, _, err := runCmd(t, newRootCmd(), "session", "resume", "--project", "no-such-project-xyz", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got resumeOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !got.NoHandoff {
		t.Errorf("no_handoff should be true on no-match, got envelope: %s", out)
	}
	if got.SessionID != "" {
		t.Errorf("session_id should be empty on no-match, got %q", got.SessionID)
	}
}

func TestSessionList_ProjectFilter(t *testing.T) {
	vault := setupVault(t)
	writeSessionFixtureWithProject(t, vault, "list-anvil", "list-anvil", "Anvil", "anvil", "## Handoff\n\n**Objective.** anvil\n")
	writeSessionFixtureWithProject(t, vault, "list-burgh", "list-burgh", "Burgh", "burgh", "## Handoff\n\n**Objective.** burgh\n")
	writeSessionFixture(t, vault, "list-unscoped", "list-unscoped", "Unscoped", "## Handoff\n\n**Objective.** no project\n")

	out, _, err := runCmd(t, newRootCmd(), "session", "list", "--project", "anvil", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []sessionItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("list --project --json must be a plain array: %v\n%s", err, out)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (only anvil): %s", len(items), out)
	}
	if items[0].Project != "anvil" {
		t.Errorf("project = %q, want anvil", items[0].Project)
	}
}
