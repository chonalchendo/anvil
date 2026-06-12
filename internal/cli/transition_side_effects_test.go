package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func loadIssueDoc(t *testing.T, vault, id string) *core.Artifact {
	t.Helper()
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", id+".md"))
	if err != nil {
		t.Fatalf("load %s: %v", id, err)
	}
	return a
}

// sideFXStub captures the calls made by stubSideFX-installed fakes; tests
// opt into per-fn behavior by setting fields on the returned recorder.
type sideFXStub struct {
	listEntries map[string]worktreeInfo
	listErr     error
	addErr      error
	addCalls    []addCall
	removeErr   error
	removeCalls []removeCall
	mainRoot    string
	mainErr     error
	homeDir     string
	homeErr     error

	fetchErr      error
	fetchCalls    int
	originHEAD    string
	originHEADErr error

	viewByField  map[string][]byte
	viewByFieldE map[string]error
	checksErr    error
	checksCalls  []int
	mergeErr     error
	mergeCalls   []int
}

type (
	addCall    struct{ Dir, Path, Branch, StartPoint string }
	removeCall struct{ Dir, Path string }
)

func stubSideFX(t *testing.T) *sideFXStub {
	t.Helper()
	s := &sideFXStub{
		listEntries:  map[string]worktreeInfo{},
		mainRoot:     "/repo",
		homeDir:      "/home/anvil",
		originHEAD:   "origin/master",
		viewByField:  map[string][]byte{},
		viewByFieldE: map[string]error{},
	}

	prevList := gitWorktreeListFn
	prevAdd := gitWorktreeAddFn
	prevRemove := gitWorktreeRemoveFn
	prevMain := gitMainRootFn
	prevFetch := gitFetchOriginFn
	prevOriginHEAD := gitResolveOriginHEADFn
	prevHome := userHomeFn
	prevView := ghPRViewJSONFn
	prevChecks := ghPRChecksFn
	prevMerge := ghPRMergeFn

	gitWorktreeListFn = func() (map[string]worktreeInfo, error) {
		return s.listEntries, s.listErr
	}
	gitWorktreeAddFn = func(dir, path, branch, startPoint string) error {
		s.addCalls = append(s.addCalls, addCall{Dir: dir, Path: path, Branch: branch, StartPoint: startPoint})
		return s.addErr
	}
	gitWorktreeRemoveFn = func(dir, path string) error {
		s.removeCalls = append(s.removeCalls, removeCall{Dir: dir, Path: path})
		return s.removeErr
	}
	gitMainRootFn = func() (string, error) { return s.mainRoot, s.mainErr }
	gitFetchOriginFn = func() error {
		s.fetchCalls++
		return s.fetchErr
	}
	gitResolveOriginHEADFn = func() (string, error) { return s.originHEAD, s.originHEADErr }
	userHomeFn = func() (string, error) { return s.homeDir, s.homeErr }
	ghPRViewJSONFn = func(_ int, fields string) ([]byte, error) {
		if e := s.viewByFieldE[fields]; e != nil {
			return nil, e
		}
		if b, ok := s.viewByField[fields]; ok {
			return b, nil
		}
		return []byte("{}"), nil
	}
	ghPRChecksFn = func(num int) error {
		s.checksCalls = append(s.checksCalls, num)
		return s.checksErr
	}
	ghPRMergeFn = func(num int) error {
		s.mergeCalls = append(s.mergeCalls, num)
		return s.mergeErr
	}

	t.Cleanup(func() {
		gitWorktreeListFn = prevList
		gitWorktreeAddFn = prevAdd
		gitWorktreeRemoveFn = prevRemove
		gitMainRootFn = prevMain
		gitFetchOriginFn = prevFetch
		gitResolveOriginHEADFn = prevOriginHEAD
		userHomeFn = prevHome
		ghPRViewJSONFn = prevView
		ghPRChecksFn = prevChecks
		ghPRMergeFn = prevMerge
	})
	return s
}

func TestSlugFromIssueID(t *testing.T) {
	cases := map[string]string{
		"anvil.foo-bar":  "foo-bar",
		"demo.long.slug": "long.slug",
		"no-dot":         "no-dot",
		"trailing.":      "trailing.",
		"":               "",
	}
	for in, want := range cases {
		if got := slugFromIssueID(in); got != want {
			t.Errorf("slugFromIssueID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultWorktreePath(t *testing.T) {
	s := stubSideFX(t)
	s.homeDir = "/u/me"
	got, err := defaultWorktreePath("anvil", "foo")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/u/me", "Development", "anvil-worktrees", "foo"); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestCutWorktreeHappyPath(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")

	if len(s.addCalls) != 1 {
		t.Fatalf("want 1 add call, got %d: %+v", len(s.addCalls), s.addCalls)
	}
	got := s.addCalls[0]
	wantPath := filepath.Join(s.homeDir, "Development", "demo-worktrees", "foo")
	if got.Path != wantPath || got.Branch != "demo/foo" || got.StartPoint != "origin/master" {
		t.Errorf("add called with %+v; want path=%s branch=demo/foo startPoint=origin/master", got, wantPath)
	}
	if s.fetchCalls != 1 {
		t.Errorf("want 1 fetch call, got %d", s.fetchCalls)
	}
	// Verify state advanced.
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress", a.FrontMatter["status"])
	}
}

func TestCutWorktreeOverridesPathAndBranch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--cut-worktree",
		"--worktree", "/tmp/wt", "--branch", "topic/x")

	if len(s.addCalls) != 1 || s.addCalls[0].Path != "/tmp/wt" || s.addCalls[0].Branch != "topic/x" {
		t.Errorf("add calls = %+v", s.addCalls)
	}
}

func TestCutWorktreeIdempotentSkipsAdd(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	wantPath := filepath.Join(s.homeDir, "Development", "demo-worktrees", "foo")
	s.listEntries["demo/foo"] = worktreeInfo{path: wantPath}

	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")

	if len(s.addCalls) != 0 {
		t.Errorf("expected zero add calls (idempotent), got %+v", s.addCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress", a.FrontMatter["status"])
	}
}

func TestCutWorktreeBranchAtWrongPathRefuses(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.listEntries["demo/foo"] = worktreeInfo{path: "/somewhere/else"}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

func TestCutWorktreeAddFailureRefusesTransition(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.addErr = errors.New("fatal: invalid reference")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

func TestCutWorktreeRejectedOnWrongEdge(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--cut-worktree", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "invalid_flag_for_transition") {
		t.Errorf("missing error code: %s", stdout.String())
	}
}

func TestWorktreeOverrideWithoutCutFlagRejected(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--worktree", "/tmp/wt", "--json",
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "invalid_flag_for_transition") {
		t.Errorf("missing error code: %s", stdout.String())
	}
}

func TestLandPRHappyPath(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	// Provide a worktree for the PR's head branch so removal proceeds.
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(s.checksCalls) != 1 || s.checksCalls[0] != 42 {
		t.Errorf("checks calls = %v", s.checksCalls)
	}
	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v", s.mergeCalls)
	}
	// Audit line written.
	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "resolved --land-pr 42") {
		t.Errorf("audit line missing in body:\n%s", body)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

func TestLandPRRefusesWhenNotMergeable(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"CONFLICTING","mergeStateStatus":"DIRTY"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_not_mergeable") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress (unchanged)", a.FrontMatter["status"])
	}
}

func TestLandPRRefusesWhenCINotGreen(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE"}`)
	s.checksErr = errors.New("check `tests` failed")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_ci_not_green") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
}

func TestLandPRRefusesWhenFinalStateNotMerged(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE"}`)
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`)
	// Provide a worktree so removal proceeds before the state check.
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_state_not_merged") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress (unchanged)", a.FrontMatter["status"])
	}
}

func TestLandPRRejectedOnWrongEdge(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--land-pr", "1", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "invalid_flag_for_transition") {
		t.Errorf("missing error code: %s", stdout.String())
	}
}

func TestLandPRConflictsWithForce(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "1", "--force", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "flags_conflict") {
		t.Errorf("missing error code: %s", stdout.String())
	}
}

func TestLandPRRefusesWhenWorktreeRemoveFails(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.homeDir = t.TempDir()
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE"}`)
	// Make the derived worktree path exist so the remove branch fires.
	wtPath := filepath.Join(s.homeDir, "Development", "demo-worktrees", "foo")
	if err := os.MkdirAll(wtPath, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	s.removeErr = errors.New("uncommitted changes in worktree")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_worktree_remove_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress (unchanged)", a.FrontMatter["status"])
	}
}

func TestCutWorktreeRejectedWhenIDLacksDot(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// Hand-author an issue with id missing the project prefix; createDemoIssue
	// always emits `demo.foo`, so we drop a malformed file in place.
	body := `---
type: issue
title: x
description: x
goal: x is done
created: 2026-05-19
status: open
project: ""
severity: low
tags: [domain/dev-tools]
---
body
`
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "nodot.md"), []byte(body), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "nodot", "in-progress", "--owner", "claude", "--cut-worktree", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_path_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
}

// TestCutWorktreeFetchFailureFallsBackToLocalHEAD verifies that a fetch error
// is non-fatal: the worktree is still cut, but startPoint is empty (local HEAD)
// and a warning is printed.
func TestCutWorktreeFetchFailureFallsBackToLocalHEAD(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.fetchErr = errors.New("network unreachable")

	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")

	if len(s.addCalls) != 1 {
		t.Fatalf("want 1 add call, got %d: %+v", len(s.addCalls), s.addCalls)
	}
	if s.addCalls[0].StartPoint != "" {
		t.Errorf("startPoint = %q; want empty (fallback to local HEAD)", s.addCalls[0].StartPoint)
	}
	if !strings.Contains(out, "warning: git fetch origin failed") || !strings.Contains(out, "branching from local HEAD") {
		t.Errorf("missing fetch-failure warning in output: %s", out)
	}
}

// TestCutWorktreeOriginHEADResolutionFailureFallsBackToLocalHEAD verifies that
// when origin/HEAD cannot be resolved (e.g. remote has no HEAD set), the
// worktree is still cut from local HEAD without error — and the fallback is
// warned about, not silent.
func TestCutWorktreeOriginHEADResolutionFailureFallsBackToLocalHEAD(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.originHEADErr = errors.New("origin/HEAD not set")

	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")

	if len(s.addCalls) != 1 {
		t.Fatalf("want 1 add call, got %d: %+v", len(s.addCalls), s.addCalls)
	}
	if s.addCalls[0].StartPoint != "" {
		t.Errorf("startPoint = %q; want empty (fallback to local HEAD)", s.addCalls[0].StartPoint)
	}
	if !strings.Contains(out, "warning: resolving origin/HEAD failed") || !strings.Contains(out, "branching from local HEAD") {
		t.Errorf("missing origin/HEAD-fallback warning in output: %s", out)
	}
}

// TestCutWorktreeEmitsWorktreePath verifies the claim emits the worktree path
// in both human and JSON modes — the completing-issue skill's "emits the
// worktree path; cd into it" contract.
func TestCutWorktreeEmitsWorktreePath(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	wantPath := filepath.Join(s.homeDir, "Development", "demo-worktrees", "foo")

	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")
	if !strings.Contains(out, "worktree: "+wantPath) {
		t.Errorf("human output missing worktree path line: %s", out)
	}

	execCmd(t, "transition", "issue", "demo.foo", "open")
	jsonOut := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--json")
	var res transitionResult
	if err := jsonUnmarshal(t, jsonOut, &res); err != nil {
		t.Fatalf("unmarshal %q: %v", jsonOut, err)
	}
	if res.Worktree != wantPath {
		t.Errorf("json worktree = %q, want %q", res.Worktree, wantPath)
	}
}

func TestCutWorktreeRefusesWhenHomeLookupFails(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.homeErr = errors.New("HOME not set")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_path_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

func TestLandPRSaveFailureSurfacesRecovery(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	// Provide a worktree so removal proceeds before the save-failure path.
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	// Make the issue file read-only after load succeeds: LoadArtifact reads,
	// then a.Save() WriteFile fails on the unwritable inode. Restore perms in
	// cleanup so t.TempDir's RemoveAll succeeds.
	issuePath := filepath.Join(vault, "70-issues", "demo.foo.md")
	if err := os.Chmod(issuePath, 0o444); err != nil { //nolint:gosec // 0444 makes the markdown issue fixture read-only so a.Save() WriteFile fails
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(issuePath, 0o644) }) //nolint:gosec // 0644 restores writability of the markdown fixture so t.TempDir cleanup can remove it

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_succeeded_save_failed") {
		t.Errorf("missing structured code: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "anvil set issue demo.foo status resolved") {
		t.Errorf("missing recovery hint: %s", stdout.String())
	}
}

func TestClassifyPRChecks(t *testing.T) {
	checkErr := errors.New("exit status 1")
	cases := []struct {
		name    string
		out     string
		err     error
		wantErr bool
	}{
		{"all-required-pass", "all checks passing", nil, false},
		{"no-required-checks", "no required checks reported on the 'anvil/foo' branch", checkErr, false},
		{"required-failing", "check `tests` failed", checkErr, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyPRChecks(tc.out, tc.err)
			if (err != nil) != tc.wantErr {
				t.Errorf("classifyPRChecks(%q, %v) err = %v, wantErr %v", tc.out, tc.err, err, tc.wantErr)
			}
		})
	}
}

// A trailing --json must be honored, not swallowed as the --land-pr value:
// `--land-pr 42 --json` lands the PR and emits the JSON envelope.
func TestLandPRHonorsTrailingJSONFlag(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	// Provide a worktree so removal proceeds.
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	if !strings.Contains(out.String(), `"status":"transitioned"`) {
		t.Errorf("--json not honored; got: %s", out.String())
	}
}

// TestLandPRDetectsWorktreeViaHeadBranch verifies that when the default
// worktree path does not exist, landPR falls back to the live worktree list
// keyed by the PR's headRefName — the fleet case where the worktree was cut
// at a non-issue-slug path.
func TestLandPRDetectsWorktreeViaHeadBranch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	// homeDir has no Development/demo-worktrees/foo directory, so os.Stat will
	// fail and the code must fall back to the worktree list.
	s.homeDir = t.TempDir()
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	// PR branch points to a worktree at a custom slug (fleet scenario).
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/fleet-custom-slug"}`)
	s.listEntries["anvil/fleet-custom-slug"] = worktreeInfo{path: "/worktrees/fleet-custom-slug"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	if len(s.removeCalls) != 1 || s.removeCalls[0].Path != "/worktrees/fleet-custom-slug" {
		t.Errorf("remove calls = %v, want [{/repo /worktrees/fleet-custom-slug}]", s.removeCalls)
	}
}

// TestLandPRErrorsWhenNoWorktreeFound verifies that --land-pr returns
// land_pr_worktree_missing and does not merge when no worktree is found via
// either the default path or the live worktree list.
func TestLandPRErrorsWhenNoWorktreeFound(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.homeDir = t.TempDir() // no worktree directory on disk
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// headRefName returns a branch with no matching worktree entry.
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/no-worktree-branch"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_worktree_missing") {
		t.Errorf("expected land_pr_worktree_missing, got: %s", stdout.String())
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge must not be called when worktree is missing: %v", s.mergeCalls)
	}
}

// TestLandPRWorktreeOverride verifies that --worktree is honored by --land-pr
// to supply the worktree path directly.
func TestLandPRWorktreeOverride(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	// Create the override path on disk so os.Stat succeeds.
	wtPath := filepath.Join(t.TempDir(), "my-custom-worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--worktree", wtPath)

	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	if len(s.removeCalls) != 1 || s.removeCalls[0].Path != wtPath {
		t.Errorf("remove calls = %v, want path %s", s.removeCalls, wtPath)
	}
}
