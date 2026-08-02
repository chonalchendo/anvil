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
	listDirs               []string
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
	fetchDirs     []string
	originHEAD    string
	originHEADErr error

	repoDir     string
	repoDirErr  error
	repoDirArgs []string

	viewByField  map[string][]byte
	viewByFieldE map[string]error
	// viewSeq entries are consumed one per call before viewByField is
	// consulted, so a field can return e.g. state OPEN (pre-check) then
	// MERGED (post-merge verify) across successive reads.
	viewSeq           map[string][][]byte
	viewCalls         []string
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
		listEntries: map[string]worktreeInfo{},
		// A real directory: landPR os.Chdir's here before worktree removal.
		mainRoot:     t.TempDir(),
		homeDir:      "/home/anvil",
		originHEAD:   "origin/master",
		viewByField:  map[string][]byte{},
		viewByFieldE: map[string]error{},
		viewSeq:      map[string][][]byte{},
	}

	// landPR os.Chdir's to mainRoot; restore cwd so it doesn't leak to later
	// tests that resolve paths (e.g. go.mod) relative to the working dir.
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	prevList := gitWorktreeListFn
	prevAdd := gitWorktreeAddFn
	prevRemove := gitWorktreeRemoveFn
	prevLocalBranchDelete := gitDeleteLocalBranchFn
	prevMain := gitMainRootFn
	prevFetch := gitFetchOriginFn
	prevOriginHEAD := gitResolveOriginHEADFn
	prevResolveRepo := resolveProjectRepoFn
	prevHome := userHomeFn
	prevView := ghPRViewJSONFn
	prevChecks := ghPRChecksFn
	prevMerge := ghPRMergeFn
	prevDeleteBranch := ghDeleteBranchFn
	prevSleep := mergeabilityPollSleep

	gitWorktreeListFn = func(dir string) (map[string]worktreeInfo, error) {
		s.listDirs = append(s.listDirs, dir)
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
	gitFetchOriginFn = func(dir string) error {
		s.fetchCalls++
		s.fetchDirs = append(s.fetchDirs, dir)
		return s.fetchErr
	}
	gitResolveOriginHEADFn = func(_ string) (string, error) { return s.originHEAD, s.originHEADErr }
	// Default mirrors the `~/Development/<project>` convention used by
	// defaultWorktreePath, but from a static homeDir rather than userHomeFn
	// (so tests exercising homeErr aren't coupled to repo resolution).
	resolveProjectRepoFn = func(project string) (string, error) {
		s.repoDirArgs = append(s.repoDirArgs, project)
		if s.repoDirErr != nil {
			return "", s.repoDirErr
		}
		if s.repoDir != "" {
			return s.repoDir, nil
		}
		return filepath.Join(s.homeDir, "Development", project), nil
	}
	userHomeFn = func() (string, error) { return s.homeDir, s.homeErr }
	ghPRViewJSONFn = func(_ int, fields string) ([]byte, error) {
		s.viewCalls = append(s.viewCalls, fields)
		if seq := s.viewSeq[fields]; len(seq) > 0 {
			s.viewSeq[fields] = seq[1:]
			return seq[0], nil
		}
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
		resolveProjectRepoFn = prevResolveRepo
		userHomeFn = prevHome
		ghPRViewJSONFn = prevView
		ghPRChecksFn = prevChecks
		ghPRMergeFn = prevMerge
		ghDeleteBranchFn = prevDeleteBranch
		mergeabilityPollSleep = prevSleep
		_ = os.Chdir(origWd)
	})
	return s
}

// countViewCalls counts the stub's gh-pr-view reads for one field set,
// distinguishing mergeability-poll traffic from the pre-check/verify reads.
func countViewCalls(calls []string, fields string) int {
	n := 0
	for _, f := range calls {
		if f == fields {
			n++
		}
	}
	return n
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

func TestCutWorktreeCarriesDeclaredUntrackedFile(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("FRED_API_KEY=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "run.sh"), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // G306: executable fixture must stay executable
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, carryFileName), []byte("# comment\n.env\nrun.sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")

	execCmd(t, "transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--cut-worktree", "--worktree", wtPath)

	got, err := os.ReadFile(filepath.Join(wtPath, ".env")) //nolint:gosec // G304: test-controlled temp path, not user input
	if err != nil {
		t.Fatalf("expected .env carried into worktree: %v", err)
	}
	if string(got) != "FRED_API_KEY=secret\n" {
		t.Errorf("carried .env content = %q, want %q", got, "FRED_API_KEY=secret\n")
	}
	info, err := os.Stat(filepath.Join(wtPath, "run.sh"))
	if err != nil {
		t.Fatalf("expected run.sh carried into worktree: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("carried run.sh mode = %o, want 700 (source mode preserved)", info.Mode().Perm())
	}
}

func TestCutWorktreeCarryEscapingPathRefuses(t *testing.T) {
	for _, entry := range []string{"../outside.env", "/etc/passwd"} {
		t.Run(entry, func(t *testing.T) {
			vault := t.TempDir()
			t.Setenv("ANVIL_VAULT", vault)
			execCmd(t, "init", vault)
			createDemoIssue(t)

			repoDir := filepath.Join(t.TempDir(), "repo")
			if err := os.MkdirAll(repoDir, 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repoDir, carryFileName), []byte(entry+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			s := stubSideFX(t)
			s.repoDir = repoDir
			wtPath := filepath.Join(t.TempDir(), "wt")

			cmd := newRootCmd()
			cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--worktree", wtPath, "--json"})
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("expected refusal; stdout: %s", stdout.String())
			}
			if !strings.Contains(stdout.String(), "cut_worktree_carry_invalid") {
				t.Errorf("missing cut_worktree_carry_invalid code: %s", stdout.String())
			}
			if len(s.addCalls) != 0 {
				t.Errorf("refusal must precede the cut; got add calls %+v", s.addCalls)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(wtPath), "outside.env")); !os.IsNotExist(err) {
				t.Errorf("escape entry must not write outside the worktree: stat err = %v", err)
			}
		})
	}
}

func TestCutWorktreeReusedWorktreeSkipsCarry(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("REPO_COPY=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, carryFileName), []byte(".env\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wtPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".env"), []byte("WORKTREE_LOCAL=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.listEntries["demo/foo"] = worktreeInfo{path: wtPath}

	execCmd(t, "transition", "issue", "demo.foo", "in-progress",
		"--owner", "claude", "--cut-worktree", "--worktree", wtPath)

	got, err := os.ReadFile(filepath.Join(wtPath, ".env")) //nolint:gosec // G304: test-controlled temp path, not user input
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "WORKTREE_LOCAL=1\n" {
		t.Errorf("re-claim must not overwrite worktree-local .env; got %q", got)
	}
}

func TestCutWorktreeCarryMissingPathRefuses(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, carryFileName), []byte(".env\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := stubSideFX(t)
	s.repoDir = repoDir
	wtPath := filepath.Join(t.TempDir(), "wt")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--worktree", wtPath, "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_carry_missing") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.addCalls) != 0 {
		t.Errorf("refusal must precede the cut (no orphan worktree); got add calls %+v", s.addCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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

// TestLandPRAlreadyResolvedNotMergedFailsExitNonZero pins anvil.0214: the
// from==to short-circuit for an already-resolved issue must not report exit
// 0 for --land-pr unless the named PR is actually merged, since a batch
// driver reads exit 0 as "the PR landed."
//
// Runs without --json purely to assert the human path; --json now returns
// non-nil too (anvil.0219), and the envelope shape is covered by the sibling
// view-failed test.
func TestLandPRAlreadyResolvedNotMergedFailsExitNonZero(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	s := stubSideFX(t)
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected a non-nil error (exit non-zero); stdout: %s", stdout.String())
	}
	if !strings.Contains(err.Error(), "land_pr_already_resolved_not_merged") {
		t.Errorf("missing error code: %v", err)
	}
	// convention.cli-tooling rule 5: the refusal names a copy-pasteable
	// recovery. resolved → open is the only legal exit, so it is a two-hop.
	wantCorrected := `anvil transition issue issue.demo.foo open --reason "<why>" && anvil transition issue issue.demo.foo resolved --land-pr 42`
	if !strings.Contains(err.Error(), wantCorrected) {
		t.Errorf("missing corrected invocation %q in:\n%v", wantCorrected, err)
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved (unchanged)", a.FrontMatter["status"])
	}
}

// TestLandPRAlreadyResolvedViewFailsRefusedJSON covers the reproduction from
// the issue: a nonexistent PR number (`gh pr view` errors) must not be
// swallowed by the already_in_state short-circuit either. This is the --json
// envelope half — `phase` disambiguates this gate's land_pr_view_failed from
// landPR's, which means "land aborted, issue still open".
func TestLandPRAlreadyResolvedViewFailsRefusedJSON(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	s := stubSideFX(t)
	s.viewByFieldE["state"] = errors.New("exit status 1")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "999999", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_view_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"phase":"already_resolved_verify"`) {
		t.Errorf("missing phase discriminator: %s", stdout.String())
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved (unchanged)", a.FrontMatter["status"])
	}
}

// TestLandPRAlreadyResolvedGhUnavailableNamesEscape pins the offline case:
// unlike the open-PR resolve check, this gate fails closed when gh is missing
// (it IS the verification), so the refusal must name the escape rather than
// leave the operator stuck.
func TestLandPRAlreadyResolvedGhUnavailableNamesEscape(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	s := stubSideFX(t)
	s.viewByFieldE["state"] = errGhUnavailable

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected a non-nil error (exit non-zero); stdout: %s", stdout.String())
	}
	if !strings.Contains(err.Error(), "anvil transition issue issue.demo.foo resolved` still exits 0") {
		t.Errorf("gh-unavailable refusal does not name the escape:\n%v", err)
	}
}

// TestLandPRAlreadyResolvedMergedSucceeds pins the idempotent-safe case: a
// re-run against an already-resolved issue whose PR is genuinely merged
// exits 0 without re-merging (--land-pr never merges twice).
func TestLandPRAlreadyResolvedMergedSucceeds(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	s := stubSideFX(t)
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)

	out := execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json")
	if !strings.Contains(out, `"already_in_state"`) {
		t.Fatalf("expected already_in_state, got %s", out)
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

// TestLandPRAutoClaimsOpenIssueWithOwner covers the dispatched-worker land
// path: the issue is never claimed in-progress, and --land-pr --owner claims
// and resolves it in one call rather than refusing with illegal_transition.
func TestLandPRAutoClaimsOpenIssueWithOwner(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	// Issue is left open — no prior `in-progress --owner` claim.

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--owner", "claude")

	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
	if a.FrontMatter["owner"] != "claude" {
		t.Errorf("owner = %v, want claude", a.FrontMatter["owner"])
	}
}

// TestLandPRAutoClaimsOpenIssueWithoutOwner asserts --land-pr alone (no
// --owner) is enough to auto-claim an unclaimed open issue: --land-pr is
// itself the explicit opt-in, matching the issue's non-goal that only the
// unclaimed-open case auto-claims (not an owner-gated variant of it).
func TestLandPRAutoClaimsOpenIssueWithoutOwner(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
	if a.FrontMatter["owner"] != landClaimOwner {
		t.Errorf("owner = %v, want %s (default claimant records who landed)", a.FrontMatter["owner"], landClaimOwner)
	}
}

// TestLandPRRefusesAbandonedIssue pins the from == "open" conjunct of the
// auto-claim guard: only open auto-claims, so an abandoned issue still takes
// the ordinary illegal-transition refusal and never reaches the merge.
func TestLandPRRefusesAbandonedIssue(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "abandoned")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "illegal_transition") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "abandoned" {
		t.Errorf("status = %v, want abandoned (unchanged)", a.FrontMatter["status"])
	}
}

// TestLandPRLeavesInProgressOwnerUntouched covers the non-goal: auto-claim
// applies only to the unclaimed-open case. An issue already in-progress
// under another owner takes the normal in-progress → resolved edge, and
// --land-pr must not overwrite its existing owner.
func TestLandPRLeavesInProgressOwnerUntouched(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "alice")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
	if a.FrontMatter["owner"] != "alice" {
		t.Errorf("owner = %v, want alice (untouched by land-pr)", a.FrontMatter["owner"])
	}
}

// TestLandPRDoesNotReassignOwnerOnClaimedIssue is the load-bearing half of the
// non-goal above: an explicit --owner on the land call must not silently
// reassign an issue someone else already claimed. Only the auto-claimed
// open case stamps an owner.
func TestLandPRDoesNotReassignOwnerOnClaimedIssue(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "alice")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--owner", "bob")

	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
	if a.FrontMatter["owner"] != "alice" {
		t.Errorf("owner = %v, want alice (--owner bob must not reassign a live claim)", a.FrontMatter["owner"])
	}
}

// TestLandPRAutoClaimFailedMergeLeavesIssueOpen asserts atomicity: an
// auto-claim attempt whose land fails a gate leaves the issue open, not
// stuck claimed-but-unresolved.
func TestLandPRAutoClaimFailedMergeLeavesIssueOpen(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`) // pre-check passes; the mergeability gate refuses
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"CONFLICTING","mergeStateStatus":"DIRTY"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--owner", "claude", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_not_mergeable") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v, want open (unchanged; claim never persisted)", a.FrontMatter["status"])
	}
	if _, ok := a.FrontMatter["owner"]; ok {
		t.Errorf("owner = %v, want unset (claim never persisted)", a.FrontMatter["owner"])
	}
}

func TestLandPRRefusesWhenNotMergeable(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`) // pre-check passes; the mergeability gate refuses
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"CONFLICTING","mergeStateStatus":"DIRTY"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// First poll returns UNKNOWN; the second returns MERGEABLE — land-pr must
	// poll past the transient rather than hard-abort on the first read.
	s.viewSeq["mergeable,mergeStateStatus"] = [][]byte{[]byte(`{"mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN"}`)}
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if polls := countViewCalls(s.viewCalls, "mergeable,mergeStateStatus"); polls < 2 {
		t.Errorf("expected at least 2 mergeability polls (UNKNOWN then MERGEABLE), got %d", polls)
	}
	if len(s.mergeCalls) != 1 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

// TestLandPRRetryOfAlreadyMergedPRSucceedsWithoutPolling covers anvil.0220: a
// PR that merged during an interrupted batch run also reads mergeable:UNKNOWN
// (GitHub never recomputes mergeability for a closed PR), so a re-run of
// --land-pr against it must recognise "already landed" up front rather than
// exhausting the mergeability poll budget and refusing land_pr_not_mergeable.
func TestLandPRRetryOfAlreadyMergedPRSucceedsWithoutPolling(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	// mergeable stays UNKNOWN forever, matching a real already-MERGED PR;
	// the pre-check must short-circuit before this ever gets polled.
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	// Warrants for the negative call assertions: if the alreadyMerged
	// short-circuit regressed, landPR would burn the poll on the permanent
	// UNKNOWN (observable only as poll reads), re-run the CI gate, or call
	// `gh pr merge` on a closed PR — which errors for real.
	if polls := countViewCalls(s.viewCalls, "mergeable,mergeStateStatus"); polls != 0 {
		t.Errorf("mergeability must not be polled for an already-merged PR: %d polls", polls)
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called for an already-merged PR: %v", s.mergeCalls)
	}
	if len(s.checksCalls) != 0 {
		t.Errorf("CI checks should not have been run for an already-merged PR: %v", s.checksCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || len(s.deleteBranchCalls) != 1 {
		t.Errorf("expected cleanup to still run: local=%v remote=%v", s.localBranchDeleteCalls, s.deleteBranchCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

// TestLandPRRetryAlreadyMergedWorktreeGoneStillResolves covers anvil.0220's
// second consequence: the interrupted run may have removed the worktree before
// dying, so the retry must treat "no worktree found" as already cleaned — warn
// and continue to branch cleanup and issue resolution. If the alreadyMerged
// bypass of the land_pr_worktree_missing abort regressed, this fails with the
// issue stranded in-progress.
func TestLandPRRetryAlreadyMergedWorktreeGoneStillResolves(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.homeDir = t.TempDir() // default worktree path does not exist
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	// No listEntries entry either: the interrupted run already removed it.

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("retry of an already-merged PR must not abort on a missing worktree: %v", err)
	}
	if !strings.Contains(stderr.String(), "worktree not found") {
		t.Errorf("expected an already-removed warning, got: %s", stderr.String())
	}
	if len(s.removeCalls) != 0 {
		t.Errorf("no worktree to remove: %v", s.removeCalls)
	}
	if len(s.localBranchDeleteCalls) != 1 || len(s.deleteBranchCalls) != 1 {
		t.Errorf("expected branch cleanup to still run: local=%v remote=%v", s.localBranchDeleteCalls, s.deleteBranchCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

// TestLandPRRefusesClosedPRWithoutPolling: a CLOSED-without-merge PR also
// reads mergeable:UNKNOWN and no amount of polling changes that, so the
// pre-check refuses immediately with the observed state. If the CLOSED branch
// regressed, the land would burn the full poll budget and misreport
// land_pr_not_mergeable/UNKNOWN instead of naming the real problem.
func TestLandPRRefusesClosedPRWithoutPolling(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["state"] = []byte(`{"state":"CLOSED"}`)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_state_not_merged") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "CLOSED") {
		t.Errorf("refusal must carry the observed state: %s", stdout.String())
	}
	if polls := countViewCalls(s.viewCalls, "mergeable,mergeStateStatus"); polls != 0 {
		t.Errorf("a closed PR must refuse before any mergeability poll: %d polls", polls)
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge should not have been called: %v", s.mergeCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "in-progress" {
		t.Errorf("status = %v, want in-progress (unchanged)", a.FrontMatter["status"])
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
	// The PR stays OPEN and every poll returns UNKNOWN — mergeability never
	// resolves.
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_not_mergeable") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if polls := countViewCalls(s.viewCalls, "mergeable,mergeStateStatus"); polls < 2 {
		t.Errorf("expected multiple mergeability polls before abort, got %d", polls)
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
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`) // pre-check passes; the CI gate refuses
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE"}`)
	s.checksErr = errors.New("check `tests` failed")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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

func TestLandPRTransientMergeRaceIsRetryable(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"BEHIND"}`)
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: "/worktrees/foo"}
	// master moved under the PR between the mergeability check and the merge.
	s.mergeErr = errors.New("gh pr merge: exit status 1: GraphQL: Base branch was modified. Review and try the merge again")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	// Distinct retryable code, not the generic land_pr_merge_failed.
	if !strings.Contains(stdout.String(), "land_pr_base_modified") {
		t.Errorf("missing retryable race code: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "land_pr_merge_failed") {
		t.Errorf("transient race must not surface as the generic terminal code: %s", stdout.String())
	}
	// Retryable in place: worktree intact, issue still in-progress.
	if len(s.removeCalls) != 0 {
		t.Errorf("worktree must survive a transient race: %v", s.removeCalls)
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)} // pre-check
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_path_failed") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

// TestCutWorktreeResolvesRepoFromProject verifies anvil.0184: the worktree is
// cut from the issue's project repo (resolved via resolveProjectRepoFn), not
// from whatever repo the caller's ambient cwd happens to belong to. Every
// repo-touching call — worktree add, fetch, origin/HEAD resolution — must
// carry the resolved repoDir, not "" (which git would resolve from cwd).
func TestCutWorktreeResolvesRepoFromProject(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.repoDir = filepath.Join(t.TempDir(), "Development", "demo")

	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")

	if len(s.repoDirArgs) != 1 || s.repoDirArgs[0] != "demo" {
		t.Fatalf("resolveProjectRepoFn calls = %v, want one call for project %q", s.repoDirArgs, "demo")
	}
	if len(s.addCalls) != 1 || s.addCalls[0].Dir != s.repoDir {
		t.Fatalf("add call dir = %q, want resolved repo %q (calls: %+v)", s.addCalls[0].Dir, s.repoDir, s.addCalls)
	}
	if len(s.fetchDirs) != 1 || s.fetchDirs[0] != s.repoDir {
		t.Errorf("fetch dir = %v, want [%s]", s.fetchDirs, s.repoDir)
	}
}

// TestCutWorktreeListsFromResolvedRepo pins the idempotency pre-check to the
// resolved project repo. Listing from the caller's ambient cwd let a
// same-branch worktree in the *caller's* repo short-circuit the cut, so the
// command returned a wrong-repo worktree as success — the silent path
// anvil.0184's goal rules out.
func TestCutWorktreeListsFromResolvedRepo(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.repoDir = filepath.Join(t.TempDir(), "Development", "demo")

	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree")

	if len(s.listDirs) != 1 || s.listDirs[0] != s.repoDir {
		t.Fatalf("worktree list dirs = %v, want [%s] (ambient cwd listing reopens the wrong-repo cut)", s.listDirs, s.repoDir)
	}
}

// TestResolveProjectRepoAcceptsCwdWhenItIsTheProject covers a checkout living
// outside the `~/Development/<project>` convention: the cwd is accepted only
// when its toplevel basename matches the project, so a correct-repo caller is
// not refused while an arbitrary cwd still is.
func TestResolveProjectRepoAcceptsCwdWhenItIsTheProject(t *testing.T) {
	prevHome, prevTop := userHomeFn, gitToplevelFn
	t.Cleanup(func() { userHomeFn, gitToplevelFn = prevHome, prevTop })
	userHomeFn = func() (string, error) { return t.TempDir(), nil } // no Development/demo

	elsewhere := filepath.Join(t.TempDir(), "src", "demo")
	gitToplevelFn = func() (string, error) { return elsewhere, nil }
	got, err := resolveProjectRepoReal("demo")
	if err != nil || got != elsewhere {
		t.Fatalf("resolveProjectRepoReal(demo) = %q, %v; want %q, nil", got, err, elsewhere)
	}

	gitToplevelFn = func() (string, error) { return filepath.Join(t.TempDir(), "src", "mentat"), nil }
	if _, err := resolveProjectRepoReal("demo"); err == nil {
		t.Fatal("resolveProjectRepoReal(demo) from a mentat cwd = nil error, want refusal")
	}
}

// TestCutWorktreeRefusesWhenRepoUnresolved verifies anvil.0184's refusal
// path: when the issue's project repo cannot be resolved (missing directory,
// not a git repo), the transition refuses loudly instead of silently cutting
// from the caller's cwd repo.
func TestCutWorktreeRefusesWhenRepoUnresolved(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	s := stubSideFX(t)
	s.repoDirErr = errors.New("project repo not found (expected `~/Development/demo`)")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cut_worktree_repo_unresolved") {
		t.Errorf("missing error code: %s", stdout.String())
	}
	if len(s.addCalls) != 0 {
		t.Errorf("expected zero add calls when repo unresolved, got %+v", s.addCalls)
	}
	a := loadIssueDoc(t, vault, "demo.foo")
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v after refusal, want open (unchanged)", a.FrontMatter["status"])
	}
}

func TestResolveProjectRepoRealRefusesMissingDir(t *testing.T) {
	prevHome := userHomeFn
	t.Cleanup(func() { userHomeFn = prevHome })
	userHomeFn = func() (string, error) { return t.TempDir(), nil }

	if _, err := resolveProjectRepoReal("does-not-exist"); err == nil {
		t.Error("want error for a project with no `~/Development/<project>` dir")
	}
}

func TestResolveProjectRepoRealRefusesNonGitDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Development", "plain"), 0o750); err != nil {
		t.Fatal(err)
	}
	prevHome := userHomeFn
	t.Cleanup(func() { userHomeFn = prevHome })
	userHomeFn = func() (string, error) { return home, nil }

	if _, err := resolveProjectRepoReal("plain"); err == nil {
		t.Error("want error for a directory without .git")
	}
}

func TestResolveProjectRepoRealResolvesGitDir(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "Development", "demo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}
	prevHome := userHomeFn
	t.Cleanup(func() { userHomeFn = prevHome })
	userHomeFn = func() (string, error) { return home, nil }

	got, err := resolveProjectRepoReal("demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != repo {
		t.Errorf("resolveProjectRepoReal = %q, want %q", got, repo)
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
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)} // pre-check
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "land_pr_succeeded_save_failed") {
		t.Errorf("missing structured code: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "anvil set issue issue.demo.foo status resolved") {
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	// PR branch points to a worktree at a custom slug (fleet scenario).
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/fleet-custom-slug"}`)
	s.listEntries["anvil/fleet-custom-slug"] = worktreeInfo{path: "/worktrees/fleet-custom-slug"}

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v, want [42]", s.mergeCalls)
	}
	if len(s.removeCalls) != 1 || s.removeCalls[0].Path != "/worktrees/fleet-custom-slug" {
		t.Errorf("remove calls = %v, want one with Path /worktrees/fleet-custom-slug", s.removeCalls)
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	s.homeDir = t.TempDir()                             // no worktree directory on disk
	s.viewByField["state"] = []byte(`{"state":"OPEN"}`) // an OPEN PR keeps worktree-missing fatal
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// headRefName returns a branch with no matching worktree entry.
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/no-worktree-branch"}`)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// anvil.0219: a failing --json invocation now returns non-nil.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
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

// TestLandPRChdirsToRootBeforeWorktreeRemoval guards anvil.0131: --land-pr is
// naturally fired from inside the issue worktree. landPR must move cwd to root
// before removing that worktree, so substeps inheriting cwd (ghDeleteBranchFn,
// the post-doLandPR open-PR check) don't shell out from the deleted directory
// and fail with "Unable to read current working directory".
func TestLandPRChdirsToRootBeforeWorktreeRemoval(t *testing.T) {
	root := t.TempDir()
	deadCwd := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(deadCwd); err != nil {
		t.Fatal(err)
	}

	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}

	s := stubSideFX(t)
	s.mainRoot = root
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	// The pre-check reads OPEN; the post-merge verify falls through to MERGED.
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField["headRefName"] = []byte(`{"headRefName":"anvil/foo"}`)
	s.listEntries["anvil/foo"] = worktreeInfo{path: deadCwd}

	// Simulate the real worktree removal destroying the process's original cwd.
	gitWorktreeRemoveFn = func(_, _ string) error { return os.RemoveAll(deadCwd) }
	// A substep that inherits cwd: record where it actually runs. os.Getwd in a
	// Go parent stays resolvable after the dir is unlinked (only a subprocess
	// like gh fails), so assert the cwd is root via SameFile — without the fix
	// it is the now-removed worktree and the Stat fails.
	var substepErr error
	substepAtRoot := false
	ghDeleteBranchFn = func(_ string) error {
		wd, e := os.Getwd()
		if e != nil {
			substepErr = e
			return e
		}
		info, e := os.Stat(wd)
		if e != nil {
			substepErr = e
			return e
		}
		substepAtRoot = os.SameFile(rootInfo, info)
		return nil
	}

	if err := landPR(&bytes.Buffer{}, 42, deadCwd, false); err != nil {
		t.Fatalf("landPR returned error: %v", err)
	}
	if substepErr != nil {
		t.Errorf("branch-delete substep saw an unresolvable cwd: %v", substepErr)
	}
	if !substepAtRoot {
		t.Errorf("branch-delete substep did not run from root %s", root)
	}
}
