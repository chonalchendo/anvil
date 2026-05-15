package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestParseWorktreePorcelain_MultipleEntries(t *testing.T) {
	in := strings.Join([]string{
		"worktree /a/main",
		"HEAD aaa111",
		"branch refs/heads/master",
		"",
		"worktree /a/worktrees/foo",
		"HEAD bbb222",
		"branch refs/heads/anvil/foo",
		"",
		"worktree /a/worktrees/bare",
		"HEAD ccc333",
		"detached",
		"",
	}, "\n")
	got := parseWorktreePorcelain(in)
	want := map[string]worktreeInfo{
		"master":    {path: "/a/main", headSHA: "aaa111"},
		"anvil/foo": {path: "/a/worktrees/foo", headSHA: "bbb222"},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(worktreeInfo{})); diff != "" {
		t.Errorf("parseWorktreePorcelain (-want +got):\n%s", diff)
	}
}

func TestRollupCI(t *testing.T) {
	cases := []struct {
		name string
		in   []ghStatusCheck
		want string
	}{
		{"empty", nil, ""},
		{"all-success", []ghStatusCheck{{Conclusion: "SUCCESS", Status: "COMPLETED"}}, "success"},
		{"one-failure-wins", []ghStatusCheck{
			{Conclusion: "SUCCESS", Status: "COMPLETED"},
			{Conclusion: "FAILURE", Status: "COMPLETED"},
		}, "failure"},
		{"pending-beats-success", []ghStatusCheck{
			{Conclusion: "SUCCESS", Status: "COMPLETED"},
			{Conclusion: "", Status: "IN_PROGRESS"},
		}, "pending"},
		{"cancelled-is-failure", []ghStatusCheck{
			{Conclusion: "CANCELLED", Status: "COMPLETED"},
		}, "failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rollupCI(tc.in); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// TestBuildFleetRows_MatchesIssuesToWorktrees stubs out every shell-out and
// verifies the matching path: in-progress issues with a candidate branch
// that names a known worktree get filled rows; others get a "no matching
// worktree" note. Open-status issues are skipped entirely.
func TestBuildFleetRows_MatchesIssuesToWorktrees(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)

	// In-progress issue with matching worktree.
	writeIssue(t, vault, "anvil.matched-issue", "in-progress", "claude-alpha")
	// In-progress issue with no worktree.
	writeIssue(t, vault, "anvil.orphan-issue", "in-progress", "claude-beta")
	// Open issue — should be skipped.
	writeIssue(t, vault, "anvil.not-claimed", "open", "")

	// Stub shell-outs.
	t.Cleanup(swapFleetStubs(
		map[string]worktreeInfo{
			"anvil/matched-issue": {path: "/tmp/wt/matched-issue", headSHA: "deadbee"},
		},
		map[string]string{"/tmp/wt/matched-issue": "ahead-2"},
		map[string]*ghPRSnapshot{
			"/tmp/wt/matched-issue": {
				Number: 42, URL: "https://github.com/x/y/pull/42",
				Mergeable: "MERGEABLE", ReviewDecision: "APPROVED",
				CIConclusion: "success",
			},
		},
		map[int]int{42: 3},
	))

	v := &core.Vault{Root: vault}
	rows, err := buildFleetRows(v)
	if err != nil {
		t.Fatalf("buildFleetRows: %v", err)
	}

	byID := map[string]fleetRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if len(byID) != 2 {
		t.Fatalf("want 2 in-progress rows, got %d: %+v", len(byID), rows)
	}
	got := byID["anvil.matched-issue"]
	want := fleetRow{
		ID: "anvil.matched-issue", Owner: "claude-alpha",
		Worktree: "/tmp/wt/matched-issue", Branch: "anvil/matched-issue",
		HeadSHA: "deadbee", PushState: "ahead-2",
		PRNumber: 42, PRURL: "https://github.com/x/y/pull/42",
		PRMergeable: "MERGEABLE", CIConclusion: "success",
		ReviewerState: "APPROVED", OpenInlineComments: 3,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("matched row (-want +got):\n%s", diff)
	}
	if note := byID["anvil.orphan-issue"].Note; note != "no matching worktree" {
		t.Errorf("orphan row note = %q, want 'no matching worktree'", note)
	}
	if _, present := byID["anvil.not-claimed"]; present {
		t.Errorf("open issue should not appear in fleet rows")
	}
}

func TestUniqueSubstringWorktree(t *testing.T) {
	wts := map[string]worktreeInfo{
		"anvil/fleet-status-verb": {path: "/wt/a", headSHA: "aaa"},
		"anvil/something-else":    {path: "/wt/b", headSHA: "bbb"},
		"master":                  {path: "/main", headSHA: "ccc"},
	}
	b, wt, ok := uniqueSubstringWorktree(wts, "anvil.anvil-fleet-status-verb-single-shot-view-of-in-flight-issues")
	if !ok || b != "anvil/fleet-status-verb" || wt.path != "/wt/a" {
		t.Errorf("got (%q, %+v, %v); want fleet-status-verb match", b, wt, ok)
	}

	// Ambiguous: two branches substring-match.
	wts2 := map[string]worktreeInfo{
		"anvil/foo":     {path: "/wt/foo"},
		"anvil/foo-bar": {path: "/wt/foo-bar"},
	}
	if _, _, ok := uniqueSubstringWorktree(wts2, "anvil.foo-bar-baz"); ok {
		t.Error("expected ambiguous (>1 substring) to return ok=false")
	}

	// No worktrees → no match.
	if _, _, ok := uniqueSubstringWorktree(map[string]worktreeInfo{}, "anvil.x"); ok {
		t.Error("expected empty map to return ok=false")
	}
}

func TestFleetStatus_JSONEnvelope(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	writeIssue(t, vault, "anvil.solo-issue", "in-progress", "claude-x")
	t.Cleanup(swapFleetStubs(nil, nil, nil, nil))

	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "fleet", "status", "--json")
	if err != nil {
		t.Fatalf("fleet status: %v", err)
	}
	var env fleetEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if env.Count != 1 || len(env.Rows) != 1 {
		t.Errorf("want count=1, got %+v", env)
	}
	if env.Rows[0].ID != "anvil.solo-issue" {
		t.Errorf("row id = %q", env.Rows[0].ID)
	}
}

// writeIssue is a minimal fixture: issue with status + owner only. Title
// matters because candidateBranchesForIssue uses the slug portion of the id.
func writeIssue(t *testing.T, vault, id, status, owner string) {
	t.Helper()
	fm := map[string]any{
		"type": "issue", "title": id, "description": "fixture",
		"created": "2026-05-15", "updated": "2026-05-15",
		"status": status, "project": "anvil", "severity": "medium",
		"tags": []any{"domain/cli"},
	}
	if owner != "" {
		fm["owner"] = owner
	}
	a := &core.Artifact{
		Path:        filepath.Join(vault, "70-issues", id+".md"),
		FrontMatter: fm,
		Body:        "## Problem\n\nfixture.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// swapFleetStubs installs canned shell-out responses and returns the
// teardown that restores the originals. nil maps are treated as empty.
func swapFleetStubs(
	worktrees map[string]worktreeInfo,
	pushState map[string]string,
	prs map[string]*ghPRSnapshot,
	comments map[int]int,
) func() {
	origWT, origPush, origPR, origCom := gitWorktreeListFn, gitPushStateFn, ghPRViewFn, ghPRCommentsFn
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) {
		if worktrees == nil {
			return map[string]worktreeInfo{}, nil
		}
		return worktrees, nil
	}
	gitPushStateFn = func(dir string) (string, error) {
		if s, ok := pushState[dir]; ok {
			return s, nil
		}
		return "", nil
	}
	ghPRViewFn = func(dir, _ string) (*ghPRSnapshot, error) {
		return prs[dir], nil
	}
	ghPRCommentsFn = func(_ string, n int) (int, error) {
		return comments[n], nil
	}
	return func() {
		gitWorktreeListFn, gitPushStateFn, ghPRViewFn, ghPRCommentsFn = origWT, origPush, origPR, origCom
	}
}
