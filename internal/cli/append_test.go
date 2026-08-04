package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeFixtureInbox writes a minimal inbox artifact and returns its path.
func writeFixtureInbox(t *testing.T, vault, id, title, created string) string {
	t.Helper()
	path := filepath.Join(vault, "00-inbox", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "inbox", "title": title,
			"created": created, "updated": created, "status": "raw",
		},
		Body: "fixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAppend_Inbox_AppendsBodyAndBumpsUpdated pins the two effects append
// exists to give agents over a raw `cat >>` edit: the new section lands and
// `updated` moves, in one atomic write.
func TestAppend_Inbox_AppendsBodyAndBumpsUpdated(t *testing.T) {
	vault := setupVault(t)
	path := writeFixtureInbox(t, vault, "2026-01-01-probe", "probe", "2026-01-01")

	bodyFile := filepath.Join(t.TempDir(), "addendum.md")
	if err := os.WriteFile(bodyFile, []byte("## Addendum\n\nnew content.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	if _, stderr, err := runCmd(t, cmd, "append", "inbox", "2026-01-01-probe", "--body-file", bodyFile); err != nil {
		t.Fatalf("append: %v\n%s", err, stderr)
	}

	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Body, "fixture body.") || !strings.Contains(a.Body, "## Addendum") {
		t.Errorf("body missing original or appended content: %q", a.Body)
	}
	if a.FrontMatter["updated"] == "2026-01-01" {
		t.Error("updated was not bumped")
	}
}

// TestAppend_UnresolvedWikilink_RejectsAndDoesNotWrite pins the validation
// gate: an addendum carrying a dangling wikilink is rejected the same way
// `create` rejects one, and the original file is left untouched — the whole
// point of routing appends through the CLI instead of a raw file edit.
func TestAppend_UnresolvedWikilink_RejectsAndDoesNotWrite(t *testing.T) {
	vault := setupVault(t)
	path := writeFixtureInbox(t, vault, "2026-01-01-probe", "probe", "2026-01-01")
	before, err := os.ReadFile(path) //nolint:gosec // G304: test-controlled temp path, not user input
	if err != nil {
		t.Fatal(err)
	}

	bodyFile := filepath.Join(t.TempDir(), "bad.md")
	if err := os.WriteFile(bodyFile, []byte("## Bad\n\n[[thread.does-not-exist-zzz]]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	if _, _, err := runCmd(t, cmd, "append", "inbox", "2026-01-01-probe", "--body-file", bodyFile); err == nil {
		t.Fatal("expected unresolved-wikilink rejection")
	}

	after, err := os.ReadFile(path) //nolint:gosec // G304: test-controlled temp path, not user input
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("file was written despite validation failure")
	}
}

// TestAppend_NoContent_Errors asserts the CLI-level guard (missing both
// --body and --body-file) fires before touching the vault, matching create's
// non-interactive contract — no prompt, an actionable stderr error instead.
func TestAppend_NoContent_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureInbox(t, vault, "2026-01-01-probe", "probe", "2026-01-01")

	cmd := newRootCmd()
	if _, _, err := runCmd(t, cmd, "append", "inbox", "2026-01-01-probe"); err == nil {
		t.Fatal("expected error for missing --body/--body-file")
	}
}
