package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	listEntries            map[string]worktreeInfo
	listErr                error
	addErr                 error
	addCalls               []addCall
	removeErr              error
	removeCalls            []removeCall
	localBranchDeleteErr   error
	localBranchDeleteCalls []localBranchDeleteCall
	mainRoot               string
	mainErr                error
	homeDir                string
	homeErr                error

	fetchErr      error
	fetchCalls    int
	originHEAD    string
	originHEADErr error

	viewByField       map[string][]byte
	viewByFieldE      map[string]error
	checksErr         error
	checksCalls       []int
	mergeErr          error
	mergeCalls        []int
	deleteBranchErr   error
	deleteBranchCalls []string
}

type (
	addCall               struct{ Dir, Path, Branch, StartPoint string }
	removeCall            struct{ Dir, Path string }
	localBranchDeleteCall struct{ Dir, Branch string }
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
	prevLocalBranchDelete := gitDeleteLocalBranchFn
	prevMain := gitMainRootFn
	prevFetch := gitFetchOriginFn
	prevOriginHEAD := gitResolveOriginHEADFn
	prevHome := userHomeFn
	prevView := ghPRViewJSONFn
	prevChecks := ghPRChecksFn
	prevMerge := ghPRMergeFn
	prevDeleteBranch := ghDeleteBranchFn
	prevSleep := mergeabilityPollSleep

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
	gitDeleteLocalBranchFn = func(dir, branch string) error {
		s.localBranchDeleteCalls = append(s.localBranchDeleteCalls, localBranchDeleteCall{Dir: dir, Branch: branch})
		return s.localBranchDeleteErr
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
	ghDeleteBranchFn = func(branch string) error {
		s.deleteBranchCalls = append(s.deleteBranchCalls, branch)
		return s.deleteBranchErr
	}
	// Suppress real sleeps in tests; tests that care about poll behavior
	// override ghPRViewJSONFn directly after calling stubSideFX.
	mergeabilityPollSleep = func(_ time.Duration) {}

	t.Cleanup(func() {
		gitWorktreeListFn = prevList
		gitWorktreeAddFn = prevAdd
		gitWorktreeRemoveFn = prevRemove
		gitDeleteLocalBranchFn = prevLocalBranchDelete
		gitMainRootFn = prevMain
		gitFetchOriginFn = prevFetch
		gitResolveOriginHEADFn = prevOriginHEAD
		userHomeFn = prevHome
		ghPRViewJSONFn = prevView
		ghPRChecksFn = prevChecks
		ghPRMergeFn = prevMerge
		ghDeleteBranchFn = prevDeleteBranch
		mergeabilityPollSleep = prevSleep
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
	// Parity with the old `--delete-branch`: a successful land deletes the local
	// branch (from the main root) and the remote branch.
	if len(s.localBranchDeleteCalls) != 1 || s.localBranchDeleteCalls[0].Branch != "anvil/foo" || s.localBranchDeleteCalls[0].Dir != s.mainRoot {
		t.Errorf("local branch delete calls = %v, want one {%s anvil/foo}", s.localBranchDeleteCalls, s.mainRoot)
	}
	if len(s.deleteBranchCalls) != 1 || s.deleteBranchCalls[0] != "anvil/foo" {
		t.Errorf("remote branch delete calls = %v, want [anvil/foo]", s.deleteBranchCalls)
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

// TestLandPRPollsUnknownMergeability verifies that a transient UNKNOWN
// mergeability is polled past rather than hard-aborted on the first read.
func TestLandPRPollsUnknownMergeability(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	// First call returns UNKNOWN; second returns MERGEABLE — land-pr must succeed.
	callCount := 0
	ghPRViewJSONFn = func(_ int, fields string) ([]byte, error) {
		if fields == "mergeable,mergeStateStatus" {
			callCount++
			if callCount == 1 {
				return []byte(`{"mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN"}`), nil
			}
			return []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`), nil
		}
		if b, ok := s.viewByField[fields]; ok {
			return b, nil
		}
		return []byte("{}"), nil
	}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if callCount < 2 {
		t.Errorf("expected at least 2 mergeability polls (UNKNOWN then MERGEABLE), got %d", callCount)
	}
	if len(s.mergeCalls) != 1 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

// Guards anvil.0078's non-goal: polling past a transient UNKNOWN must not
// bypass the mergeability gate. If every poll stays UNKNOWN the verb still
// hard-aborts rather than merging on unresolved mergeability.
func TestLandPRRefusesWhenMergeabilityNeverResolves(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	// Every poll returns UNKNOWN — mergeability never resolves.
	callCount := 0
	ghPRViewJSONFn = func(_ int, fields string) ([]byte, error) {
		if fields == "mergeable,mergeStateStatus" {
			callCount++
			return []byte(`{"mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN"}`), nil
		}
		if b, ok := s.viewByField[fields]; ok {
			return b, nil
		}
		return []byte("{}"), nil
	}

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
	if callCount < 2 {
		t.Errorf("expected multiple mergeability polls before abort, got %d", callCount)
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
	// Atomicity guarantee: a non-MERGED state leaves the worktree and branches
	// untouched, so a clean retry is possible after the merge failure is fixed.
	if len(s.removeCalls) != 0 {
		t.Errorf("worktree must not be removed on non-MERGED state: %v", s.removeCalls)
	}
	if len(s.deleteBranchCalls) != 0 {
		t.Errorf("remote branch must not be deleted on non-MERGED state: %v", s.deleteBranchCalls)
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
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// Merge succeeds and the PR state confirms MERGED; only the worktree removal
	// fails (e.g. uncommitted changes). The new order is merge → verify → remove,
	// so merge must have been called before the remove is attempted.
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
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
	// Merge is called before the remove in the new ordering.
	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge must be called before remove: %v", s.mergeCalls)
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

// TestLandPRLocalValidatedBypassesCICheck confirms that --local-validated
// skips ghPRChecksFn even when it would return an error, allowing the merge
// to proceed. This covers the operator-attestation override path: required CI
// is unavailable but the operator has verified locally.
func TestLandPRLocalValidatedBypassesCICheck(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}
	// CI would fail without the override.
	s.checksErr = errors.New("check `tests` failed: timed out waiting for status")

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--local-validated")

	// Merge must have been called despite the CI error.
	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	// CI check must have been skipped entirely.
	if len(s.checksCalls) != 0 {
		t.Errorf("CI check should be skipped with --local-validated; got calls: %v", s.checksCalls)
	}
}

// TestLandPRLocalValidatedAuditsOverride pins the audit trail: a land that
// bypasses the CI gate with --local-validated must record that fact in the
// issue body so a later reader sees the attestation.
func TestLandPRLocalValidatedAuditsOverride(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}
	s.checksErr = errors.New("check `tests` failed")

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--local-validated")

	a := loadIssueDoc(t, vault, "demo.foo")
	if !strings.Contains(a.Body, "--local-validated") {
		t.Errorf("audit line missing --local-validated tag:\n%s", a.Body)
	}
	if !strings.Contains(a.Body, "CI gate bypassed") {
		t.Errorf("audit line missing bypass notice:\n%s", a.Body)
	}
}

// TestLocalValidatedWithoutLandPRRejected ensures --local-validated cannot be
// used without --land-pr: using the flag alone is a user error and must
// return an explicit rejection rather than silently being ignored.
func TestLocalValidatedWithoutLandPRRejected(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--local-validated", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "invalid_flag_for_transition") {
		t.Errorf("expected invalid_flag_for_transition rejection; got: %s", stdout.String())
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

// TestLandPRMergesBeforeRemovingWorktree verifies that the merge call precedes
// the worktree-remove call. This ordering prevents the "Unable to read current
// working directory" failure that occurs when cwd is inside the worktree being
// removed.
func TestLandPRMergesBeforeRemovingWorktree(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	var callOrder []string
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"demo/foo"}`)
	s.listEntries["demo/foo"] = worktreeInfo{path: "/worktrees/foo"}

	prevMerge := ghPRMergeFn
	prevRemove := gitWorktreeRemoveFn
	t.Cleanup(func() {
		ghPRMergeFn = prevMerge
		gitWorktreeRemoveFn = prevRemove
	})
	ghPRMergeFn = func(num int) error {
		callOrder = append(callOrder, "merge")
		s.mergeCalls = append(s.mergeCalls, num)
		return nil
	}
	gitWorktreeRemoveFn = func(dir, path string) error {
		callOrder = append(callOrder, "remove")
		s.removeCalls = append(s.removeCalls, removeCall{Dir: dir, Path: path})
		return nil
	}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(callOrder) < 2 || callOrder[0] != "merge" || callOrder[1] != "remove" {
		t.Errorf("call order = %v; want [merge remove ...]", callOrder)
	}
}

// TestLandPRMergeExitNonZeroButStateMerged verifies that when ghPRMergeFn
// returns an error but the live PR state is MERGED (post-merge checkout
// failure from inside a worktree), landPR treats it as success.
func TestLandPRMergeExitNonZeroButStateMerged(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"demo/foo"}`)
	s.listEntries["demo/foo"] = worktreeInfo{path: "/worktrees/foo"}
	// Simulate gh pr merge exiting non-zero (post-merge checkout failure).
	s.mergeErr = errors.New("exit status 1: failed to run git: fatal: Unable to read current working directory")

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(s.mergeCalls) != 1 {
		t.Errorf("merge must have been called: %v", s.mergeCalls)
	}
	// Worktree removal, local + remote branch delete must still proceed.
	if len(s.removeCalls) != 1 {
		t.Errorf("worktree must be removed after confirmed-MERGED: %v", s.removeCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || s.localBranchDeleteCalls[0].Branch != "demo/foo" {
		t.Errorf("local branch delete must be called after confirmed-MERGED: %v", s.localBranchDeleteCalls)
	}
	if len(s.deleteBranchCalls) != 1 {
		t.Errorf("branch delete must be called after confirmed-MERGED: %v", s.deleteBranchCalls)
	}
}

// TestLandPRResolvesDespiteBranchDeleteFailure guards anvil.0110: once the PR is
// confirmed MERGED, a failing branch-delete substep (e.g. the remote ref is
// already gone) must not abort the land. The merge already landed, so cleanup is
// best-effort — the verb warns and drives the issue to resolved rather than
// stranding a merged PR with an in-progress issue.
func TestLandPRResolvesDespiteBranchDeleteFailure(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}
	// Simulate the remote branch-delete failing post-merge (ref already gone).
	s.deleteBranchErr = errors.New("gh api delete branch: exit status 1")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("land-pr must not abort on branch-delete failure: %v\nstderr: %s", err, stderr.String())
	}
	// Atomicity: the merge landed and the issue reaches resolved.
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved (atomic land despite branch-delete failure)", a.FrontMatter["status"])
	}
	// The failure is surfaced as a warning, not swallowed.
	if !strings.Contains(stderr.String(), "remote branch delete failed") {
		t.Errorf("expected a branch-delete-failure warning, got: %s", stderr.String())
	}
}
