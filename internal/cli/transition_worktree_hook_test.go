package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHookScript(t *testing.T, repoDir, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, worktreeHookName), []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestCutWorktreeRunsHookAfterCarry(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	writeHookScript(t, repoDir, "#!/bin/sh\nprintf hooked > \"$1/.hook-ran\"\n", 0o700) //nolint:gosec // G306: fixture must stay executable

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")
	// A real `git worktree add` creates the directory; the test stubs that call
	// out, so pre-create it to model the state the hook expects.
	if err := os.MkdirAll(wtPath, 0o750); err != nil {
		t.Fatal(err)
	}

	execCmd(t, "transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--cut-worktree", "--worktree", wtPath)

	got, err := os.ReadFile(filepath.Join(wtPath, ".hook-ran")) //nolint:gosec // G304: test-controlled temp path
	if err != nil {
		t.Fatalf("expected hook to run: %v", err)
	}
	if string(got) != "hooked" {
		t.Errorf("hook marker = %q, want %q", got, "hooked")
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress", a.FrontMatter["status"])
	}
}

func TestCutWorktreeMissingHookIsNoop(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.repoDir = t.TempDir() // no .anvil-worktree-hook present
	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wtPath, 0o750); err != nil {
		t.Fatal(err)
	}

	execCmd(t, "transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--cut-worktree", "--worktree", wtPath)

	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress (missing hook must not block the cut)", a.FrontMatter["status"])
	}
}

func TestCutWorktreeHookNotExecutableRefusesAndCleansUp(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	writeHookScript(t, repoDir, "#!/bin/sh\nexit 0\n", 0o600)

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wtPath, 0o750); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--worktree", wtPath, "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_hook_not_executable") {
		t.Errorf("missing cut_worktree_hook_not_executable code: %s", stdout.String())
	}
	if len(s.removeCalls) != 1 || s.removeCalls[0].Path != wtPath {
		t.Errorf("expected worktree removed on hook refusal; removeCalls = %+v", s.removeCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || s.localBranchDeleteCalls[0].Branch != "demo/foo" {
		t.Errorf("expected branch deleted on hook refusal; calls = %+v", s.localBranchDeleteCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

func TestCutWorktreeHookFailureRefusesAndCleansUp(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	writeHookScript(t, repoDir, "#!/bin/sh\necho boom >&2\nexit 1\n", 0o700) //nolint:gosec // G306: fixture must stay executable

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wtPath, 0o750); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--worktree", wtPath, "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_hook_failed") {
		t.Errorf("missing cut_worktree_hook_failed code: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "boom") {
		t.Errorf("expected stderr tail carried in refusal: %s", stdout.String())
	}
	if len(s.removeCalls) != 1 || s.removeCalls[0].Path != wtPath {
		t.Errorf("expected worktree removed on hook failure; removeCalls = %+v", s.removeCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || s.localBranchDeleteCalls[0].Branch != "demo/foo" {
		t.Errorf("expected branch deleted on hook failure; calls = %+v", s.localBranchDeleteCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

func TestCutWorktreeReusedWorktreeSkipsHook(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	writeHookScript(t, repoDir, "#!/bin/sh\nprintf hooked > \"$1/.hook-ran\"\n", 0o700) //nolint:gosec // G306: fixture must stay executable

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wtPath, 0o750); err != nil {
		t.Fatal(err)
	}
	s.listEntries["demo/foo"] = worktreeInfo{path: wtPath}

	execCmd(t, "transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--cut-worktree", "--worktree", wtPath)

	if _, err := os.Stat(filepath.Join(wtPath, ".hook-ran")); !os.IsNotExist(err) {
		t.Errorf("expected hook skipped on reuse; stat err = %v", err)
	}
}
