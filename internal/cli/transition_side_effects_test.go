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

	viewByField  map[string][]byte
	viewByFieldE map[string]error
	checksErr    error
	checksCalls  []int
	mergeErr     error
	mergeCalls   []int
}

type (
	addCall    struct{ Dir, Path, Branch string }
	removeCall struct{ Dir, Path string }
)

func stubSideFX(t *testing.T) *sideFXStub {
	t.Helper()
	s := &sideFXStub{
		listEntries:  map[string]worktreeInfo{},
		mainRoot:     "/repo",
		homeDir:      "/home/anvil",
		viewByField:  map[string][]byte{},
		viewByFieldE: map[string]error{},
	}

	prevList := gitWorktreeListFn
	prevAdd := gitWorktreeAddFn
	prevRemove := gitWorktreeRemoveFn
	prevMain := gitMainRootFn
	prevHome := userHomeFn
	prevView := ghPRViewJSONFn
	prevChecks := ghPRChecksFn
	prevMerge := ghPRMergeFn

	gitWorktreeListFn = func() (map[string]worktreeInfo, error) {
		return s.listEntries, s.listErr
	}
	gitWorktreeAddFn = func(dir, path, branch string) error {
		s.addCalls = append(s.addCalls, addCall{Dir: dir, Path: path, Branch: branch})
		return s.addErr
	}
	gitWorktreeRemoveFn = func(dir, path string) error {
		s.removeCalls = append(s.removeCalls, removeCall{Dir: dir, Path: path})
		return s.removeErr
	}
	gitMainRootFn = func() (string, error) { return s.mainRoot, s.mainErr }
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
	if got.Path != wantPath || got.Branch != "demo/foo" {
		t.Errorf("add called with %+v; want path=%s branch=demo/foo", got, wantPath)
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "cut_worktree_failed") {
		t.Errorf("missing error code: %s", out.String())
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "cut_worktree_failed") {
		t.Errorf("missing error code: %s", out.String())
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--cut-worktree"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "invalid_flag_for_transition") {
		t.Errorf("missing error code: %s", out.String())
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
		"--owner", "claude", "--worktree", "/tmp/wt",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "invalid_flag_for_transition") {
		t.Errorf("missing error code: %s", out.String())
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

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")

	if len(s.checksCalls) != 1 || s.checksCalls[0] != 42 {
		t.Errorf("checks calls = %v", s.checksCalls)
	}
	if len(s.mergeCalls) != 1 || s.mergeCalls[0] != 42 {
		t.Errorf("merge calls = %v", s.mergeCalls)
	}
	// Audit line written.
	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "land_pr_not_mergeable") {
		t.Errorf("missing error code: %s", out.String())
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "land_pr_ci_not_green") {
		t.Errorf("missing error code: %s", out.String())
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

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "land_pr_state_not_merged") {
		t.Errorf("missing error code: %s", out.String())
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--land-pr", "1"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "invalid_flag_for_transition") {
		t.Errorf("missing error code: %s", out.String())
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "1", "--force"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "flags_conflict") {
		t.Errorf("missing error code: %s", out.String())
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
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	s.removeErr = errors.New("uncommitted changes in worktree")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "land_pr_worktree_remove_failed") {
		t.Errorf("missing error code: %s", out.String())
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
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "nodot.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "nodot", "in-progress", "--owner", "claude", "--cut-worktree"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "cut_worktree_path_failed") {
		t.Errorf("missing error code: %s", out.String())
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
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--cut-worktree"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "cut_worktree_path_failed") {
		t.Errorf("missing error code: %s", out.String())
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

	// Make the issue file read-only after load succeeds: LoadArtifact reads,
	// then a.Save() WriteFile fails on the unwritable inode. Restore perms in
	// cleanup so t.TempDir's RemoveAll succeeds.
	issuePath := filepath.Join(vault, "70-issues", "demo.foo.md")
	if err := os.Chmod(issuePath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(issuePath, 0o644) })

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected save-failed error after merge; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "land_pr_succeeded_save_failed") {
		t.Errorf("missing structured code: %s", out.String())
	}
	if !strings.Contains(out.String(), "anvil set issue demo.foo status resolved") {
		t.Errorf("missing recovery hint: %s", out.String())
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
