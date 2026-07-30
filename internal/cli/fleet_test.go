package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

// rollupCICases covers both rollup shapes in one table. GitHub Actions writes
// Check Runs (status + conclusion); CircleCI and other legacy integrations
// write Commit Statuses — the `StatusContext` cases — which carry only state.
// Reading conclusion alone pends a green StatusContext forever. Shared by
// TestRollupCI and TestWaitForPRReducer: the Go and jq reducers must classify
// every fixture identically (Go "" is the script's "none").
var rollupCICases = []struct {
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
	{"startup-failure-is-failure", []ghStatusCheck{
		{Conclusion: "STARTUP_FAILURE", Status: "COMPLETED"},
	}, "failure"},
	{"queued-is-pending", []ghStatusCheck{{Status: "QUEUED"}}, "pending"},
	{"waiting-is-pending", []ghStatusCheck{{Status: "WAITING"}}, "pending"},
	{"requested-is-pending", []ghStatusCheck{{Status: "REQUESTED"}}, "pending"},
	{"checkrun-pending-status-is-pending", []ghStatusCheck{{Status: "PENDING"}}, "pending"},
	{"skipped-is-success", []ghStatusCheck{
		{Conclusion: "SKIPPED", Status: "COMPLETED"},
	}, "success"},
	{"neutral-is-success", []ghStatusCheck{
		{Conclusion: "NEUTRAL", Status: "COMPLETED"},
	}, "success"},
	{"green-StatusContext-no-conclusion", []ghStatusCheck{
		{State: "SUCCESS"},
	}, "success"},
	{"pending-StatusContext", []ghStatusCheck{
		{State: "PENDING"},
	}, "pending"},
	{"expected-StatusContext-is-pending", []ghStatusCheck{
		{State: "EXPECTED"},
	}, "pending"},
	{"failing-StatusContext", []ghStatusCheck{
		{State: "FAILURE"},
	}, "failure"},
	{"errored-StatusContext", []ghStatusCheck{
		{State: "ERROR"},
	}, "failure"},
	{"green-CheckRun-plus-pending-StatusContext", []ghStatusCheck{
		{Conclusion: "SUCCESS", Status: "COMPLETED"},
		{State: "PENDING"},
	}, "pending"},
	{"green-CheckRun-plus-green-StatusContext", []ghStatusCheck{
		{Conclusion: "SUCCESS", Status: "COMPLETED"},
		{State: "SUCCESS"},
	}, "success"},
	{"failing-StatusContext-beats-green-CheckRun", []ghStatusCheck{
		{Conclusion: "SUCCESS", Status: "COMPLETED"},
		{State: "FAILURE"},
	}, "failure"},
	{"action-required-is-failure", []ghStatusCheck{
		{Conclusion: "ACTION_REQUIRED", Status: "COMPLETED"},
	}, "failure"},
	{"stale-is-failure", []ghStatusCheck{
		{Conclusion: "STALE", Status: "COMPLETED"},
	}, "failure"},
	{"action-required-beats-green", []ghStatusCheck{
		{Conclusion: "SUCCESS", Status: "COMPLETED"},
		{Conclusion: "ACTION_REQUIRED", Status: "COMPLETED"},
	}, "failure"},
	{"unknown-conclusion-is-blank", []ghStatusCheck{
		{Conclusion: "FUTURE_STATE", Status: "COMPLETED"},
	}, ""},
}

func TestRollupCI(t *testing.T) {
	for _, tc := range rollupCICases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rollupCI(tc.in); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// TestWaitForPRReducer shells the jq program embedded in
// anvil/skills/completing-issue/scripts/wait-for-pr.sh over rollupCICases.
// If the jq arms drift from rollupCI's classification of any state — the
// divergence that recurred across anvil.0210 and anvil.0217 while the Go
// table held — this fails on the diverging fixture. Fixtures marshal with
// empty fields omitted because gh emits typename-specific keys (a Commit
// Status has no "status" key), which is what the jq null checks read.
func TestWaitForPRReducer(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skipf("jq not on PATH: %v", err)
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "anvil", "skills", "completing-issue", "scripts", "wait-for-pr.sh"))
	if err != nil {
		t.Fatalf("read wait-for-pr.sh: %v", err)
	}
	const marker = "--json statusCheckRollup --jq '"
	_, after, ok := strings.Cut(string(script), marker)
	if !ok {
		t.Fatalf("marker %q not found in wait-for-pr.sh", marker)
	}
	prog, _, ok := strings.Cut(after, "'")
	if !ok {
		t.Fatal("unterminated jq program in wait-for-pr.sh")
	}
	for _, tc := range rollupCICases {
		t.Run(tc.name, func(t *testing.T) {
			rollup := make([]map[string]string, 0, len(tc.in))
			for _, c := range tc.in {
				entry := map[string]string{}
				if c.Conclusion != "" {
					entry["conclusion"] = c.Conclusion
				}
				if c.Status != "" {
					entry["status"] = c.Status
				}
				if c.State != "" {
					entry["state"] = c.State
				}
				rollup = append(rollup, entry)
			}
			input, err := json.Marshal(map[string]any{"statusCheckRollup": rollup})
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			cmd := exec.Command("jq", "-r", prog) //nolint:gosec // program text read from the repo-committed script, not user input
			cmd.Stdin = strings.NewReader(string(input))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("jq: %v", err)
			}
			want := tc.want
			if want == "" {
				want = "none"
			}
			if got := strings.TrimSpace(string(out)); got != want {
				t.Errorf("got %q want %q", got, want)
			}
		})
	}
}

// TestRollupCI_DecodesLiveStatusContextJSON drives the verbatim bytes
// `gh pr view 141 --repo chonalchendo/mentat --json statusCheckRollup`
// returned (2026-07-26, CircleCI green) through the same decode + reduce path
// ghPRViewReal uses. The struct-literal table above cannot catch a wrong or
// missing json tag — every case there bypasses encoding/json — so a green
// CircleCI PR would still roll up as pending with the tags broken.
func TestRollupCI_DecodesLiveStatusContextJSON(t *testing.T) {
	const live = `{"statusCheckRollup":[` +
		`{"__typename":"StatusContext","context":"ci/circleci: changes","startedAt":"2026-07-25T23:52:47Z","state":"SUCCESS","targetUrl":"https://circleci.com/gh/chonalchendo/mentat/614"},` +
		`{"__typename":"StatusContext","context":"ci/circleci: check-mentat-connectors","startedAt":"2026-07-25T23:53:39Z","state":"SUCCESS","targetUrl":"https://circleci.com/gh/chonalchendo/mentat/616"},` +
		`{"__typename":"StatusContext","context":"ci/circleci: ci-ok","startedAt":"2026-07-25T23:52:56Z","state":"SUCCESS","targetUrl":"https://circleci.com/gh/chonalchendo/mentat/615"},` +
		`{"__typename":"StatusContext","context":"ci/circleci: imports","startedAt":"2026-07-25T23:53:08Z","state":"SUCCESS","targetUrl":"https://circleci.com/gh/chonalchendo/mentat/617"}]}`

	var pr ghPRSnapshot
	if err := json.Unmarshal([]byte(live), &pr); err != nil {
		t.Fatalf("unmarshal live rollup: %v", err)
	}
	if len(pr.StatusCheckRoll) != 4 {
		t.Fatalf("decoded %d entries, want 4", len(pr.StatusCheckRoll))
	}
	if got := rollupCI(pr.StatusCheckRoll); got != "success" {
		t.Errorf("rollupCI on live CircleCI rollup = %q, want %q", got, "success")
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
	got := byID["issue.anvil.matched-issue"]
	want := fleetRow{
		ID: "issue.anvil.matched-issue", Owner: "claude-alpha",
		Worktree: "/tmp/wt/matched-issue", Branch: "anvil/matched-issue",
		HeadSHA: "deadbee", PushState: "ahead-2",
		PRNumber: 42, PRURL: "https://github.com/x/y/pull/42",
		PRMergeable: "MERGEABLE", CIConclusion: "success",
		ReviewerState: "APPROVED", OpenInlineComments: 3,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("matched row (-want +got):\n%s", diff)
	}
	if note := byID["issue.anvil.orphan-issue"].Note; note != "no matching worktree" {
		t.Errorf("orphan row note = %q, want 'no matching worktree'", note)
	}
	if _, present := byID["issue.anvil.not-claimed"]; present {
		t.Errorf("open issue should not appear in fleet rows")
	}
}

func TestGenericSlugWorktree(t *testing.T) {
	wts := map[string]worktreeInfo{
		"mentat/matched-issue": {path: "/wt/mentat", headSHA: "aaa"},
		"burgh/other-issue":    {path: "/wt/burgh", headSHA: "bbb"},
		"master":               {path: "/main", headSHA: "ccc"},
	}
	b, wt, ok := genericSlugWorktree(wts, []string{"matched-issue"})
	if !ok || b != "mentat/matched-issue" || wt.path != "/wt/mentat" {
		t.Errorf("got (%q, %+v, %v); want mentat/matched-issue match", b, wt, ok)
	}

	// No slash in branch name — not a candidate.
	if _, _, ok := genericSlugWorktree(wts, []string{"master"}); ok {
		t.Error("expected bare branch (no prefix) not to match")
	}

	// Candidates are tried in order: a miss on the first falls through to the
	// linked-plan slug rather than aborting.
	b, _, ok = genericSlugWorktree(wts, []string{"no-such-slug", "other-issue"})
	if !ok || b != "burgh/other-issue" {
		t.Errorf("got (%q, %v); want fallthrough to burgh/other-issue", b, ok)
	}

	// Ambiguous: two branches with the same trailing slug, different prefixes.
	wts2 := map[string]worktreeInfo{
		"mentat/dup-slug": {path: "/wt/a"},
		"burgh/dup-slug":  {path: "/wt/b"},
	}
	if _, _, ok := genericSlugWorktree(wts2, []string{"dup-slug"}); ok {
		t.Error("expected ambiguous (>1 prefix match) to return ok=false")
	}

	// No worktrees → no match.
	if _, _, ok := genericSlugWorktree(map[string]worktreeInfo{}, []string{"x"}); ok {
		t.Error("expected empty map to return ok=false")
	}
}

// TestBuildFleetRows_MatchesNonAnvilPrefixedBranch reproduces the mentat/burgh
// case: an in-progress issue whose worktree branch is `<project>/<slug>`
// rather than `anvil/<slug>`, resolved from a *different* project repo than
// the one this fleet status invocation ambiently runs in, must still
// resolve — not fall back to "no matching worktree".
func TestBuildFleetRows_MatchesNonAnvilPrefixedBranch(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)

	writeIssueForProject(t, vault, "mentat.matched-issue", "in-progress", "claude-alpha", "mentat")

	t.Cleanup(swapFleetStubs(
		nil, // the "" (ambient repo) worktree list is irrelevant here
		map[string]string{},
		map[string]*ghPRSnapshot{},
		map[int]int{},
	))
	resolveProjectRepoFn = func(project string) (string, error) {
		if project == "mentat" {
			return "/repos/mentat", nil
		}
		return "", nil
	}
	origWT := gitWorktreeListFn
	gitWorktreeListFn = func(repoDir string) (map[string]worktreeInfo, error) {
		if repoDir == "/repos/mentat" {
			return map[string]worktreeInfo{
				"mentat/matched-issue": {path: "/tmp/wt/mentat-matched", headSHA: "cafefeed"},
			}, nil
		}
		return map[string]worktreeInfo{}, nil
	}
	t.Cleanup(func() { gitWorktreeListFn = origWT })

	v := &core.Vault{Root: vault}
	rows, err := buildFleetRows(v)
	if err != nil {
		t.Fatalf("buildFleetRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(rows), rows)
	}
	got := rows[0]
	if got.Note == "no matching worktree" {
		t.Errorf("expected a matched worktree, got note %q", got.Note)
	}
	if got.Branch != "mentat/matched-issue" || got.Worktree != "/tmp/wt/mentat-matched" {
		t.Errorf("got branch=%q worktree=%q, want mentat/matched-issue / /tmp/wt/mentat-matched", got.Branch, got.Worktree)
	}
}

// TestBuildFleetRows_MatchesLinkedPlanSlugUnderNonAnvilPrefix pins the second
// hardcode: candidate slugs come from the linked plan as well as the id, and
// both must match under any project prefix. Reproduces burgh, whose plan
// carries a `slug:` that diverges from its issue id.
func TestBuildFleetRows_MatchesLinkedPlanSlugUnderNonAnvilPrefix(t *testing.T) {
	vault := setupVault(t)
	const (
		issueID  = "burgh.espc-sold-history-connector"
		planSlug = "espc-sold-history-connector-int-espc-sales-edinburgh"
	)
	writeIssueForProject(t, vault, issueID, "in-progress", "claude-beta", "burgh")
	writeFixturePlan(t, vault, "burgh", planSlug, "ESPC sold-history connector")
	execCmd(t, "reindex")
	execCmd(t, "link", "plan", "burgh."+planSlug, "issue", issueID)

	t.Cleanup(swapFleetStubs(nil, nil, nil, nil))
	resolveProjectRepoFn = func(string) (string, error) { return "/repos/burgh", nil }
	origWT := gitWorktreeListFn
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"burgh/" + planSlug: {path: "/tmp/wt/burgh-plan", headSHA: "beefcafe"},
		}, nil
	}
	t.Cleanup(func() { gitWorktreeListFn = origWT })

	rows, err := buildFleetRows(&core.Vault{Root: vault})
	if err != nil {
		t.Fatalf("buildFleetRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(rows), rows)
	}
	if rows[0].Branch != "burgh/"+planSlug || rows[0].Worktree != "/tmp/wt/burgh-plan" {
		t.Errorf("got branch=%q worktree=%q note=%q; want the plan-slug branch matched",
			rows[0].Branch, rows[0].Worktree, rows[0].Note)
	}
}

// TestBuildFleetRows_UnresolvedProjectRepo covers both halves of the
// degradation: the ambient-repo fallback is announced on the row rather than
// masquerading as absence, and the failing resolution is memoized so N rows
// of one project cost one `git rev-parse`, not N.
func TestBuildFleetRows_UnresolvedProjectRepo(t *testing.T) {
	vault := setupVault(t)
	writeIssueForProject(t, vault, "ghost.issue-one", "in-progress", "claude-a", "ghost")
	writeIssueForProject(t, vault, "ghost.issue-two", "in-progress", "claude-b", "ghost")

	t.Cleanup(swapFleetStubs(nil, nil, nil, nil))
	calls := 0
	resolveProjectRepoFn = func(project string) (string, error) {
		calls++
		return "", fmt.Errorf("project repo not found at ~/Development/%s", project)
	}

	rows, err := buildFleetRows(&core.Vault{Root: vault})
	if err != nil {
		t.Fatalf("buildFleetRows: %v", err)
	}
	if calls != 1 {
		t.Errorf("resolveProjectRepoFn calls = %d, want 1 (memoized per project)", calls)
	}
	want := "project repo not found (expected ~/Development/ghost); matched against ambient repo"
	for _, r := range rows {
		if r.Note != want {
			t.Errorf("row %s note = %q, want %q", r.ID, r.Note, want)
		}
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

	// Divergent short slug under a non-anvil prefix: the fallback must not be
	// blind to it just because the branch is not `anvil/`-prefixed.
	wts3 := map[string]worktreeInfo{
		"burgh/sold-history": {path: "/wt/burgh", headSHA: "ddd"},
		"master":             {path: "/main"},
	}
	b, wt, ok = uniqueSubstringWorktree(wts3, "burgh.espc-sold-history-connector")
	if !ok || b != "burgh/sold-history" || wt.path != "/wt/burgh" {
		t.Errorf("got (%q, %+v, %v); want burgh/sold-history match", b, wt, ok)
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
	if env.Rows[0].ID != "issue.anvil.solo-issue" {
		t.Errorf("row id = %q", env.Rows[0].ID)
	}
}

// writeIssue is a minimal fixture: issue with status + owner only. Title
// matters because candidateBranchesForIssue uses the slug portion of the id.
func writeIssue(t *testing.T, vault, id, status, owner string) {
	t.Helper()
	writeIssueForProject(t, vault, id, status, owner, "anvil")
}

// writeIssueForProject is writeIssue with an explicit `project` frontmatter
// field, for tests exercising per-project repo resolution.
func writeIssueForProject(t *testing.T, vault, id, status, owner, project string) {
	t.Helper()
	fm := map[string]any{
		"type": "issue", "title": id, "description": "fixture",
		"created": "2026-05-15", "updated": "2026-05-15",
		"status": status, "project": project, "severity": "medium",
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
// It stubs gitWorktreeListFn to ignore repoDir and always return
// the same fixture map — sufficient for single-project tests. Tests that
// exercise per-project repo resolution (worktreesForProject) additionally
// stub resolveProjectRepoFn themselves; the hermetic default here (a
// sentinel repo dir, no error) keeps the real filesystem out of every other
// test.
func swapFleetStubs(
	worktrees map[string]worktreeInfo,
	pushState map[string]string,
	prs map[string]*ghPRSnapshot,
	comments map[int]int,
) func() {
	origWT, origPush, origPR, origCom, origRepo := gitWorktreeListFn, gitPushStateFn, ghPRViewFn, ghPRCommentsFn, resolveProjectRepoFn
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) {
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
	// A non-empty sentinel: an empty dir means "unresolved, using the ambient
	// repo", which buildFleetRows reports on the row.
	resolveProjectRepoFn = func(project string) (string, error) {
		return "/repos/" + project, nil
	}
	return func() {
		gitWorktreeListFn, gitPushStateFn, ghPRViewFn, ghPRCommentsFn, resolveProjectRepoFn = origWT, origPush, origPR, origCom, origRepo
	}
}
