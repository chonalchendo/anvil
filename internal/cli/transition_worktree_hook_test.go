package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runGit runs a git command in dir, failing the test on error — used to build
// a real repo fixture for tests that exercise the real `git worktree remove`
// rather than the removeCalls-recording stub.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // test-controlled args
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// initGitRepo creates a real git repo at dir with one commit — enough for
// `git worktree add -b <branch>` to succeed against it.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "init")
}

// realGitWorktreeFns swaps the gitWorktree*/gitDeleteLocalBranch seams for
// their real (process-executing) implementations, restoring the stubSideFX
// fakes on cleanup. Used by tests that need the real `git worktree remove`
// effect, not just the recorded call the stub asserts.
func realGitWorktreeFns(t *testing.T) {
	t.Helper()
	prevAdd, prevList, prevFetch, prevOriginHEAD := gitWorktreeAddFn, gitWorktreeListFn, gitFetchOriginFn, gitResolveOriginHEADFn
	prevRemove, prevRemoveForce, prevBranchDelete := gitWorktreeRemoveFn, gitWorktreeRemoveForceFn, gitDeleteLocalBranchFn
	gitWorktreeAddFn = gitWorktreeAddReal
	gitWorktreeListFn = gitWorktreeListReal
	gitFetchOriginFn = gitFetchOriginReal
	gitResolveOriginHEADFn = gitResolveOriginHEADReal
	gitWorktreeRemoveFn = gitWorktreeRemoveReal
	gitWorktreeRemoveForceFn = gitWorktreeRemoveForceReal
	gitDeleteLocalBranchFn = gitDeleteLocalBranchReal
	t.Cleanup(func() {
		gitWorktreeAddFn, gitWorktreeListFn, gitFetchOriginFn, gitResolveOriginHEADFn = prevAdd, prevList, prevFetch, prevOriginHEAD
		gitWorktreeRemoveFn, gitWorktreeRemoveForceFn, gitDeleteLocalBranchFn = prevRemove, prevRemoveForce, prevBranchDelete
	})
}

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
	if len(s.removeForceCalls) != 1 || s.removeForceCalls[0].Path != wtPath {
		t.Errorf("expected worktree force-removed on hook refusal; removeForceCalls = %+v", s.removeForceCalls)
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
	if len(s.removeForceCalls) != 1 || s.removeForceCalls[0].Path != wtPath {
		t.Errorf("expected worktree force-removed on hook failure; removeForceCalls = %+v", s.removeForceCalls)
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

// TestCutWorktreeHookFailureForceRemovesRealWorktree guards anvil.0270's
// blocker finding: a plain `git worktree remove` refuses once the hook has
// written a non-gitignored file, orphaning both the worktree dir and the
// branch. This drives doCutWorktree against a real repo (real `git worktree
// add`/`remove --force`/`branch -D`, not the removeCalls-recording stub) so
// the assertion is on the actual filesystem/branch effect, not just the call.
func TestCutWorktreeHookFailureForceRemovesRealWorktree(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	// Writes a non-gitignored file into the worktree before failing — the
	// case a plain `git worktree remove` refuses on.
	writeHookScript(t, repoDir, "#!/bin/sh\nprintf derived > \"$1/derived.txt\"\nexit 1\n", 0o700) //nolint:gosec // G306: fixture must stay executable

	s := stubSideFX(t)
	s.repoDir = repoDir
	realGitWorktreeFns(t)
	wtPath := filepath.Join(t.TempDir(), "wt")
	// Real `git worktree add` creates the directory; do not pre-create it.

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

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree dir removed; stat err = %v", err)
	}
	out := bytes.Buffer{}
	branchList := exec.Command("git", "branch", "--list", "demo/foo") //nolint:gosec // test-controlled args
	branchList.Dir = repoDir
	branchList.Stdout = &out
	if err := branchList.Run(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("expected branch demo/foo deleted; git branch --list = %q", out.String())
	}

	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

// TestCutWorktreeHookTimeoutRefusesAndCleansUp guards the high finding: a
// hung hook must not wedge the transition indefinitely — including when the
// kill signal doesn't land on the sleeping process itself. The fixture backgrounds
// a long sleep before its own foreground sleep, so the process exec.CommandContext
// kills (the shell, or whatever it tail-exec's into) is not the process still
// holding the stderr pipe open afterwards; a single-command `sleep N` script
// doesn't reproduce this because the shell tail-exec's straight into it, so the
// kill lands on the sleeper directly and the bug never surfaces. Asserts wall-clock
// elapsed stays bounded by timeout+WaitDelay+slack, not by the orphan's remaining
// sleep — that bound is only possible because runWorktreeHookReal sets
// cmd.WaitDelay.
func TestCutWorktreeHookTimeoutRefusesAndCleansUp(t *testing.T) {
	prevTimeoutFn, prevWaitDelayFn := worktreeHookTimeoutFn, worktreeHookWaitDelayFn
	worktreeHookTimeoutFn = func() time.Duration { return 200 * time.Millisecond }
	worktreeHookWaitDelayFn = func() time.Duration { return 300 * time.Millisecond }
	t.Cleanup(func() {
		worktreeHookTimeoutFn = prevTimeoutFn
		worktreeHookWaitDelayFn = prevWaitDelayFn
	})

	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	// Backgrounds a 5s sleep (an orphan that outlives the killed foreground
	// process and keeps the inherited stderr pipe open) before its own 5s
	// foreground sleep, well past the shrunk timeout/waitDelay above.
	writeHookScript(t, repoDir, "#!/bin/sh\nsleep 5 &\nsleep 5\n", 0o700) //nolint:gosec // G306: fixture must stay executable

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
	start := time.Now()
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; stdout: %s", stdout.String())
	}
	elapsed := time.Since(start)
	if maxElapsed := 200*time.Millisecond + 300*time.Millisecond + 2*time.Second; elapsed > maxElapsed {
		t.Errorf("elapsed = %v, want < %v (timeout+WaitDelay+slack) — an orphaned descendant wedged Wait", elapsed, maxElapsed)
	}
	if !strings.Contains(stdout.String(), "cut_worktree_hook_timeout") {
		t.Errorf("missing cut_worktree_hook_timeout code: %s", stdout.String())
	}
	if len(s.removeForceCalls) != 1 || s.removeForceCalls[0].Path != wtPath {
		t.Errorf("expected worktree force-removed on hook timeout; removeForceCalls = %+v", s.removeForceCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || s.localBranchDeleteCalls[0].Branch != "demo/foo" {
		t.Errorf("expected branch deleted on hook timeout; calls = %+v", s.localBranchDeleteCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}
