package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScopeAuditBase builds origin main <- lower (a) <- upper (b) and audits
// from upper: --base lower sees only b; no --base flags a; an unknown base refuses.
func TestScopeAuditBase(t *testing.T) {
	bare := t.TempDir()
	gitIn(t, bare, "init", "-q", "--bare", "-b", "main", ".")
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main", ".")
	gitIn(t, dir, "remote", "add", "origin", bare)
	layer := func(branch, file string) {
		t.Helper()
		if branch != "main" {
			gitIn(t, dir, "checkout", "-qb", branch)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
		gitIn(t, dir, "add", file)
		gitIn(t, dir, "commit", "-qm", file)
		gitIn(t, dir, "push", "-q", "origin", branch)
	}
	layer("main", "m")
	layer("lower", "a")
	layer("upper", "b")
	t.Chdir(dir)

	t.Run("base-lower-is-clean", func(t *testing.T) {
		stdout, _, err := runCmd(t, newRootCmd(), "fleet", "scope-audit",
			"--declared", "b", "--changed", "b", "--base", "lower")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := strings.TrimSpace(stdout); got != "scope: clean" {
			t.Errorf("stdout = %q, want scope: clean", got)
		}
	})
	t.Run("no-base-flags-lower-layer", func(t *testing.T) {
		stdout, _, err := runCmd(t, newRootCmd(), "fleet", "scope-audit",
			"--declared", "b", "--changed", "b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := strings.TrimSpace(stdout); got != "a" {
			t.Errorf("stdout = %q, want a", got)
		}
	})
	t.Run("unknown-base-refuses", func(t *testing.T) {
		stdout, _, err := runCmd(t, newRootCmd(), "fleet", "scope-audit",
			"--declared", "b", "--changed", "b", "--base", "nope")
		if err == nil {
			t.Fatalf("want refusal, got stdout %q", stdout)
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Errorf("error should name the branch: %v", err)
		}
	})
}
