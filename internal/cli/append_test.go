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
// `updated` moves, staged through the atomic rename swap so an interrupted
// rewrite can never destroy body content the command didn't author.
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

// TestAppend_EmptyBodyFile_NamesTheFile pins the empty-file diagnosis: a
// --body-file that exists but holds zero bytes must be reported as empty, not
// misreported as the flag never having been passed.
func TestAppend_EmptyBodyFile_NamesTheFile(t *testing.T) {
	vault := setupVault(t)
	writeFixtureInbox(t, vault, "2026-01-01-probe", "probe", "2026-01-01")

	bodyFile := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(bodyFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	_, _, err := runCmd(t, cmd, "append", "inbox", "2026-01-01-probe", "--body-file", bodyFile)
	if err == nil {
		t.Fatal("expected error for empty --body-file")
	}
	if !strings.Contains(err.Error(), bodyFile) || !strings.Contains(err.Error(), "is empty") {
		t.Errorf("error must name the empty file, got: %q", err.Error())
	}
}

// TestAppend_Issue_NeverExecutesVerificationBlocks pins the boundary between
// append and the create-time feasibility gate (anvil.0196): appending to an
// issue must not run the body's Verification bash blocks. The gate's Indirect
// verdict refuses exit 0, so on an already-fixed issue (whose Indirect
// predicate now passes) firing it would refuse every append; worse, the
// blocks execute unsandboxed for what is a body edit. The fixture's blocks
// touch a marker file and exit 0 — if the gate ran, the append would be
// refused AND the marker would exist.
func TestAppend_Issue_NeverExecutesVerificationBlocks(t *testing.T) {
	vault := setupVault(t)
	marker := filepath.Join(t.TempDir(), "gate-ran")
	body := "## Problem\n\np.\n\n## Non-goals\n\nn.\n\n## Verification\n\n### Direct\n\n" +
		"```bash\ntouch " + marker + "\n```\n\n### Indirect\n\n" +
		"```bash\ntouch " + marker + "\nexit 0\n```\n\n## Links\n\n- none\n"
	path := filepath.Join(vault, core.TypeIssue.Dir(), "0001-fixed-issue.md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "issue", "title": "fixed issue", "goal": "g",
			"created": "2026-01-01", "updated": "2026-01-01", "status": "resolved",
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	bodyFile := filepath.Join(t.TempDir(), "addendum.md")
	if err := os.WriteFile(bodyFile, []byte("## Addendum\n\npost-fix note.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	if _, stderr, err := runCmd(t, cmd, "append", "issue", "0001-fixed-issue", "--body-file", bodyFile); err != nil {
		t.Fatalf("append to issue must not fire the feasibility gate: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("a Verification block was executed during append")
	}
	got, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "## Addendum") {
		t.Errorf("addendum missing from body: %q", got.Body)
	}
}

// TestAppend_IdenticalRerun_IsUnchangedNoop pins retry safety: re-running the
// exact same append (an agent retry after a lost response, or after a failed
// post-save index) exits 0 reporting "unchanged" and leaves exactly one copy
// of the section — never a duplicate.
func TestAppend_IdenticalRerun_IsUnchangedNoop(t *testing.T) {
	vault := setupVault(t)
	path := writeFixtureInbox(t, vault, "2026-01-01-probe", "probe", "2026-01-01")

	bodyFile := filepath.Join(t.TempDir(), "addendum.md")
	if err := os.WriteFile(bodyFile, []byte("## Addendum\n\nnew content.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, err := runCmd(t, newRootCmd(), "append", "inbox", "2026-01-01-probe", "--body-file", bodyFile); err != nil {
		t.Fatalf("first append: %v\n%s", err, stderr)
	}
	stdout, stderr, err := runCmd(t, newRootCmd(), "append", "inbox", "2026-01-01-probe", "--body-file", bodyFile)
	if err != nil {
		t.Fatalf("identical re-run must exit 0: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "unchanged") {
		t.Errorf("re-run must report unchanged, got: %q", stdout)
	}

	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(a.Body, "## Addendum"); n != 1 {
		t.Errorf("section duplicated: %d copies", n)
	}
}

// TestAppend_PreexistingViolation_IsPrefixed pins the refusal message when
// the stored body already carries a violation the addendum didn't introduce:
// the combined body is what gets validated, so the failure still refuses, but
// it must be labelled pre-existing — an error citing content the author never
// touched otherwise reads as a bug in append.
func TestAppend_PreexistingViolation_IsPrefixed(t *testing.T) {
	vault := setupVault(t)
	path := filepath.Join(vault, "00-inbox", "2026-01-01-probe.md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "inbox", "title": "probe",
			"created": "2026-01-01", "updated": "2026-01-01", "status": "raw",
		},
		Body: "see [[thread.does-not-exist-zzz]] for context.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	bodyFile := filepath.Join(t.TempDir(), "addendum.md")
	if err := os.WriteFile(bodyFile, []byte("## Addendum\n\nclean content.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	_, stderr, err := runCmd(t, cmd, "append", "inbox", "2026-01-01-probe", "--body-file", bodyFile)
	if err == nil {
		t.Fatal("expected refusal on pre-existing unresolved wikilink")
	}
	if !strings.Contains(stderr, "pre-existing (not introduced by this append)") {
		t.Errorf("pre-existing violation must be labelled as such, stderr: %q", stderr)
	}
}
