package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// realGitWorktreeFns swaps the gitWorktree*/gitDeleteLocalBranch seams for the real
// implementations — tests asserting the filesystem effect, not the stub's recorded call.
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

// A plain `git worktree remove` refuses once the hook has written a
// non-gitignored file. Drives doCutWorktree against a real repo (real `git
// worktree add`/`remove --force`/`branch -D`), not the removeCalls stub.
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

// A hook whose orphaned child holds the stderr pipe must still refuse within
// timeout+WaitDelay, not the child's remaining lifetime.
func TestCutWorktreeHookTimeoutRefusesAndCleansUp(t *testing.T) {
	prevTimeoutFn, prevWaitDelayFn := worktreeHookTimeoutFn, worktreeHookWaitDelayFn
	timeout := 500 * time.Millisecond
	waitDelay := 200 * time.Millisecond
	worktreeHookTimeoutFn = func() time.Duration { return timeout }
	worktreeHookWaitDelayFn = func() time.Duration { return waitDelay }
	t.Cleanup(func() {
		worktreeHookTimeoutFn = prevTimeoutFn
		worktreeHookWaitDelayFn = prevWaitDelayFn
	})

	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	// Backgrounds an orphan that inherits stderr and outlives the hook's own
	// foreground sleep — group liveness is asserted via hookStartedFn below,
	// not by reading a fixture-written pid file against the deadline.
	writeHookScript(t, repoDir, "#!/bin/sh\nsleep 100 &\nsleep 100\n", 0o700) //nolint:gosec // G306: fixture must stay executable

	var hookPid int
	prevHookStartedFn := hookStartedFn
	hookStartedFn = func(pid int) { hookPid = pid }
	t.Cleanup(func() { hookStartedFn = prevHookStartedFn })

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
	// Lower bound: the refusal can't fire before the hook's own timeout.
	if elapsed < timeout {
		t.Errorf("elapsed = %v, want >= timeout %v — refused too early", elapsed, timeout)
	}
	// Upper bound: group-kill closes the orphan's inherited stderr fd on
	// cancel, so elapsed stays near timeout, well under timeout+WaitDelay —
	// which would mean the kill missed the orphan and Wait sat out the delay.
	if maxElapsed := timeout + 1*time.Second; elapsed > maxElapsed {
		t.Errorf("elapsed = %v, want < %v (timeout+slack, well under timeout+WaitDelay=%v) — group-kill didn't reach the orphan", elapsed, maxElapsed, timeout+waitDelay)
	}
	if !strings.Contains(stdout.String(), "cut_worktree_hook_timeout") {
		t.Errorf("missing cut_worktree_hook_timeout code: %s", stdout.String())
	}

	assertGroupDead(t, hookPid)

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

// A hook that fails (not via timeout or orphaned-stderr) must still have its
// process group killed — regression test for the hoisted group-kill on any
// non-nil Run error. Race-free: Run cannot return before the pid file is
// written, since the hook exits only after writing it.
func TestCutWorktreeHookFailureKillsOrphan(t *testing.T) {
	prevWaitDelayFn := worktreeHookWaitDelayFn
	worktreeHookWaitDelayFn = func() time.Duration { return 200 * time.Millisecond }
	t.Cleanup(func() { worktreeHookWaitDelayFn = prevWaitDelayFn })

	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	// exit 1 races Wait's WaitDelay against the orphan holding stderr; shrink
	// WaitDelay so the test doesn't pay the default 5s to observe the same kill.
	writeHookScript(t, repoDir, "#!/bin/sh\nsleep 100 &\necho $! > orphan.pid\nexit 1\n", 0o700) //nolint:gosec // G306: fixture must stay executable

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

	pid := readOrphanPid(t, repoDir)
	assertOrphanDead(t, pid)
}

// A hook that exits 0 but leaves a descendant holding the stderr pipe open
// must refuse with a dedicated code, not the exec-internal WaitDelay string,
// and the descendant must not survive the refusal.
func TestCutWorktreeHookOrphanProcessRefusesAndCleansUp(t *testing.T) {
	prevTimeoutFn, prevWaitDelayFn := worktreeHookTimeoutFn, worktreeHookWaitDelayFn
	worktreeHookTimeoutFn = func() time.Duration { return 5 * time.Second }
	waitDelay := 300 * time.Millisecond
	worktreeHookWaitDelayFn = func() time.Duration { return waitDelay }
	t.Cleanup(func() {
		worktreeHookTimeoutFn = prevTimeoutFn
		worktreeHookWaitDelayFn = prevWaitDelayFn
	})

	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	// Exits 0 immediately but backgrounds an orphan that inherits stderr and
	// keeps it open — the hook succeeded, but left a daemon behind.
	writeHookScript(t, repoDir, "#!/bin/sh\nsleep 100 &\necho $! > orphan.pid\nexit 0\n", 0o700) //nolint:gosec // G306: fixture must stay executable

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
	// Bounded by WaitDelay, not the timeout — the hook process itself
	// already exited, so ctx is never cancelled; only the orphan holding the
	// pipe delays the refusal, and only by WaitDelay.
	if elapsed < waitDelay {
		t.Errorf("elapsed = %v, want >= WaitDelay %v — refused before Wait had a chance to detect the orphan", elapsed, waitDelay)
	}
	if maxElapsed := waitDelay + 3*time.Second; elapsed > maxElapsed {
		t.Errorf("elapsed = %v, want < %v (WaitDelay+slack)", elapsed, maxElapsed)
	}
	if !strings.Contains(stdout.String(), "cut_worktree_hook_orphan_process") {
		t.Errorf("missing cut_worktree_hook_orphan_process code: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "WaitDelay expired") {
		t.Errorf("leaked exec-internal error string into refusal: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "the hook must spawn nothing that outlives it") {
		t.Errorf("missing fix_hint: %s", stdout.String())
	}

	pid := readOrphanPid(t, repoDir)
	assertOrphanDead(t, pid)

	if len(s.removeForceCalls) != 1 || s.removeForceCalls[0].Path != wtPath {
		t.Errorf("expected worktree force-removed on hook orphan refusal; removeForceCalls = %+v", s.removeForceCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || s.localBranchDeleteCalls[0].Branch != "demo/foo" {
		t.Errorf("expected branch deleted on hook orphan refusal; calls = %+v", s.localBranchDeleteCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

// readOrphanPid reads the pid a fixture hook recorded for its backgrounded
// orphan — written to repoDir (not the worktree, which cleanup may remove).
func readOrphanPid(t *testing.T, repoDir string) int {
	t.Helper()
	pidBytes, err := os.ReadFile(filepath.Join(repoDir, "orphan.pid")) //nolint:gosec // G304: test-controlled temp dir
	if err != nil {
		t.Fatalf("reading orphan pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatalf("parsing orphan pid %q: %v", pidBytes, err)
	}
	return pid
}

// assertOrphanDead polls kill(pid, 0) for up to 500ms — SIGKILL delivery is
// near-instant but not synchronous with the group-kill call returning, so a
// single immediate check can catch the orphan mid-teardown.
func assertOrphanDead(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	var killErr error
	for {
		killErr = syscall.Kill(pid, 0)
		if errors.Is(killErr, syscall.ESRCH) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !errors.Is(killErr, syscall.ESRCH) {
		t.Errorf("orphan pid %d still alive after refusal (kill -0 = %v), want ESRCH — group-kill left a descendant running", pid, killErr)
	}
}

// assertGroupDead polls kill(-pid, 0) — the process-group form — for up to
// 500ms, proving every process the hook started (itself and any backgrounded
// child) is dead, without depending on a fixture-written pid file.
func assertGroupDead(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	var killErr error
	for {
		killErr = syscall.Kill(-pid, 0)
		if errors.Is(killErr, syscall.ESRCH) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !errors.Is(killErr, syscall.ESRCH) {
		t.Errorf("group %d still alive after refusal (kill -0 = %v), want ESRCH — group-kill left a descendant running", pid, killErr)
	}
}
