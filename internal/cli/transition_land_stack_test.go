package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// stackFake answers gh for a two-layer stack: PR 10 (demo/0001.low, base
// master) under PR 20 (demo/0002.high, base demo/0001.low).
type stackFake struct {
	*sideFXStub
	state        map[int]string
	heads        map[int]string
	stackCalls   []int
	mergeErr     error
	mergeHook    func()
	stateErr     bool
	defaultBr    string
	allChecks    []int
	allChecksErr error
}

func landStackSetup(t *testing.T, highRound string) (*stackFake, string) {
	t.Helper()
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess1")
	execCmd(t, "init", vault)
	for _, c := range []struct{ slug, round string }{
		{"0001.low", "## Review findings — PR 10, round 1 @ " + landTestShort + "\n\nFindings: 0\n"},
		{"0002.high", highRound},
	} {
		a := &core.Artifact{
			Path: filepath.Join(vault, "70-issues", "issue.demo."+c.slug+".md"),
			FrontMatter: map[string]any{
				"type": "issue", "title": c.slug, "description": "fixture description",
				"created": "2026-01-01", "updated": "2026-01-01",
				"status": "open", "project": "demo", "severity": "medium",
				"tags": []any{"domain/dev-tools"}, "goal": "fixture goal is done",
			},
			Body: strings.Split(fixtureIssueBody, "## Review findings")[0] + c.round,
		}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	execCmd(t, "reindex")
	for _, slug := range []string{"0001.low", "0002.high"} {
		execCmd(t, "transition", "issue", "demo."+slug, "in-progress", "--owner", "claude")
	}

	f := &stackFake{sideFXStub: stubSideFX(t), state: map[int]string{}, heads: map[int]string{}, defaultBr: "master"}
	f.homeDir = t.TempDir()
	refs := map[int]struct{ head, base string }{
		10: {"demo/0001.low", "master"},
		20: {"demo/0002.high", "demo/0001.low"},
	}
	ghPRViewJSONFn = func(num int, fields string) ([]byte, error) {
		r := refs[num]
		if f.stateErr && fields == "state" {
			return nil, fmt.Errorf("gh down")
		}
		vals := map[string]string{
			"state": "OPEN", "mergeable": "MERGEABLE", "mergeStateStatus": "CLEAN",
			"headRefOid": landTestHead, "headRefName": r.head, "baseRefName": r.base,
		}
		if s := f.state[num]; s != "" {
			vals["state"] = s
		}
		if h := f.heads[num]; h != "" {
			vals["headRefOid"] = h
		}
		var parts []string
		for _, k := range strings.Split(fields, ",") {
			parts = append(parts, fmt.Sprintf("%q:%q", k, vals[k]))
		}
		return []byte("{" + strings.Join(parts, ",") + "}"), nil
	}
	prevList, prevDefault := ghPRListFn, ghDefaultBranchFn
	// stubSideFX restores ghPRChecksFn; layer 0 gates on required checks, upper layers on all.
	ghPRChecksFn = func(num int, required bool) error {
		if required {
			return nil
		}
		f.allChecks = append(f.allChecks, num)
		return f.allChecksErr
	}
	ghDefaultBranchFn = func() (string, error) { return f.defaultBr, nil }
	ghPRListFn = func(branch, _ string) (string, error) {
		if branch == "demo/0001.low" {
			return "https://github.com/o/r/pull/10", nil
		}
		return "", nil
	}
	prevStack := ghStackMergeFn
	ghStackMergeFn = func(num int) error {
		f.stackCalls = append(f.stackCalls, num)
		if f.mergeErr != nil {
			return f.mergeErr
		}
		f.state[10], f.state[20] = "MERGED", "MERGED"
		if f.mergeHook != nil {
			f.mergeHook()
		}
		return nil
	}
	t.Cleanup(func() {
		ghPRListFn, ghStackMergeFn, ghDefaultBranchFn = prevList, prevStack, prevDefault
	})
	f.listEntries["demo/0001.low"] = worktreeInfo{path: "/worktrees/low"}
	f.listEntries["demo/0002.high"] = worktreeInfo{path: "/worktrees/high"}
	return f, vault
}

func TestLandStack(t *testing.T) {
	t.Run("lands both layers with one merge", func(t *testing.T) {
		f, vault := landStackSetup(t, "## Review findings — PR 20, round 1 @ "+landTestShort+"\n\nFindings: 0\n")
		execCmd(t, "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		if len(f.stackCalls) != 1 || f.stackCalls[0] != 20 {
			t.Fatalf("gh stack merge calls = %v, want [20]", f.stackCalls)
		}
		if len(f.mergeCalls) != 0 {
			t.Errorf("per-PR merge called: %v", f.mergeCalls)
		}
		for _, slug := range []string{"0001.low", "0002.high"} {
			a := loadIssueDoc(t, vault, "issue.demo."+slug)
			if got := a.FrontMatter["status"]; got != "resolved" {
				t.Errorf("%s status = %v, want resolved", slug, got)
			}
		}
		if len(f.removeCalls) != 2 {
			t.Errorf("worktree removals = %v, want 2", f.removeCalls)
		}
	})

	t.Run("stale review on the upper layer refuses before any merge", func(t *testing.T) {
		f, vault := landStackSetup(t, "## Review findings — PR 20, round 1 @ deadbee\n\nFindings: 0\n")
		_, stderr, err := runCmd(t, newRootCmd(), "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		if err == nil || !strings.Contains(stderr+err.Error(), "land_pr_review_stale") {
			t.Fatalf("err = %v stderr = %s, want land_pr_review_stale", err, stderr)
		}
		if !strings.Contains(stderr+err.Error(), "layer_pr") {
			t.Errorf("refusal does not name the layer PR: %s", stderr)
		}
		if len(f.stackCalls) != 0 {
			t.Errorf("gh stack merge called: %v", f.stackCalls)
		}
		a := loadIssueDoc(t, vault, "issue.demo.0001.low")
		if got := a.FrontMatter["status"]; got != "in-progress" {
			t.Errorf("lower layer status = %v, want in-progress", got)
		}
	})

	t.Run("land-pr and land-stack are exclusive", func(t *testing.T) {
		landStackSetup(t, "")
		_, stderr, err := runCmd(t, newRootCmd(), "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20", "--land-pr", "20")
		if err == nil || !strings.Contains(stderr+err.Error(), "land_flags_exclusive") {
			t.Fatalf("err = %v stderr = %s", err, stderr)
		}
	})
}

const cleanHighRound = "## Review findings — PR 20, round 1 @ " + landTestShort + "\n\nFindings: 0\n"

func landStackRefusal(t *testing.T, args ...string) string {
	t.Helper()
	_, stderr, err := runCmd(t, newRootCmd(), append([]string{"transition", "issue", "demo.0002.high", "resolved"}, args...)...)
	if err == nil {
		t.Fatalf("want a refusal, got nil; stderr = %s", stderr)
	}
	return stderr + err.Error()
}

func issueStatus(t *testing.T, vault, slug string) any {
	t.Helper()
	return loadIssueDoc(t, vault, "issue.demo."+slug).FrontMatter["status"]
}

func TestLandStackRefusals(t *testing.T) {
	t.Run("red lower layer refuses before any merge", func(t *testing.T) {
		f, vault := landStackSetup(t, cleanHighRound)
		prev := landCleanRunFn
		t.Cleanup(func() { landCleanRunFn = prev })
		landCleanRunFn = func(_ io.Writer, c landClean, _, _ string) error {
			if strings.HasSuffix(c.id, "0001.low") {
				return errfmt.NewStructured("land_pr_verification_failed").Set("issue", c.id)
			}
			return nil
		}
		out := landStackRefusal(t, "--land-stack", "20")
		if !strings.Contains(out, "land_pr_verification_failed") || !strings.Contains(out, "layer_pr") {
			t.Errorf("refusal = %s", out)
		}
		if len(f.stackCalls) != 0 || issueStatus(t, vault, "0002.high") != "in-progress" {
			t.Errorf("merge ran or status moved: calls=%v", f.stackCalls)
		}
	})

	t.Run("walk refuses when the bottom base is not the default branch", func(t *testing.T) {
		f, _ := landStackSetup(t, cleanHighRound)
		f.defaultBr = "main"
		out := landStackRefusal(t, "--land-stack", "20")
		if !strings.Contains(out, "land_stack_walk_failed") || !strings.Contains(out, "fix_hint") {
			t.Errorf("refusal = %s", out)
		}
		if len(f.stackCalls) != 0 {
			t.Errorf("gh stack merge called: %v", f.stackCalls)
		}
	})

	t.Run("upper layer gates on all checks; none reported refuses", func(t *testing.T) {
		f, _ := landStackSetup(t, cleanHighRound)
		f.allChecksErr = fmt.Errorf("no checks reported")
		out := landStackRefusal(t, "--land-stack", "20")
		if !strings.Contains(out, "land_pr_ci_not_green") {
			t.Errorf("refusal = %s", out)
		}
		if len(f.allChecks) != 1 || f.allChecks[0] != 20 {
			t.Errorf("all-checks calls = %v, want [20]", f.allChecks)
		}
		if len(f.stackCalls) != 0 {
			t.Errorf("gh stack merge called: %v", f.stackCalls)
		}
	})

	t.Run("head moved after the clean run refuses before the merge", func(t *testing.T) {
		f, _ := landStackSetup(t, cleanHighRound)
		prev := landCleanRunFn
		t.Cleanup(func() { landCleanRunFn = prev })
		landCleanRunFn = func(io.Writer, landClean, string, string) error {
			f.heads[20] = "ffffffffffffffffffffffffffffffffffffffff"
			return nil
		}
		out := landStackRefusal(t, "--land-stack", "20")
		if !strings.Contains(out, "land_pr_head_moved") || !strings.Contains(out, "layer_pr") || !strings.Contains(out, "fix_hint") {
			t.Errorf("refusal = %s", out)
		}
		if len(f.stackCalls) != 0 {
			t.Errorf("gh stack merge called: %v", f.stackCalls)
		}
	})

	t.Run("head moved during the merge resolves and audits the merged head", func(t *testing.T) {
		f, vault := landStackSetup(t, cleanHighRound)
		const moved = "ffffffffffffffffffffffffffffffffffffffff"
		f.mergeHook = func() { f.heads[20] = moved }
		execCmd(t, "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		a := loadIssueDoc(t, vault, "issue.demo.0002.high")
		if got := a.FrontMatter["status"]; got != "resolved" {
			t.Errorf("status = %v, want resolved", got)
		}
		if got := a.FrontMatter["review_head"]; got != landTestHead {
			t.Errorf("review_head = %v, want the verified head %s", got, landTestHead)
		}
		if !strings.Contains(a.Body, "merged head "+moved+" differs from verified head") {
			t.Errorf("audit line lacks the merged head:\n%s", a.Body)
		}
	})

	for _, state := range []string{"OPEN", "CLOSED"} {
		t.Run("resolved lower layer with a "+state+" PR refuses", func(t *testing.T) {
			f, vault := landStackSetup(t, cleanHighRound)
			execCmd(t, "transition", "issue", "demo.0001.low", "resolved", "--force", "--reason", "fixture")
			f.state[10] = state
			out := landStackRefusal(t, "--land-stack", "20")
			if !strings.Contains(out, "land_stack_layer_resolved_unmerged") || !strings.Contains(out, "layer_pr") || !strings.Contains(out, "fix_hint") {
				t.Errorf("refusal = %s", out)
			}
			if len(f.stackCalls) != 0 {
				t.Errorf("gh stack merge called: %v", f.stackCalls)
			}
			if got := issueStatus(t, vault, "0002.high"); got != "in-progress" {
				t.Errorf("upper status = %v, want in-progress", got)
			}
		})
	}

	t.Run("walk stops at the default branch before any PR list call", func(t *testing.T) {
		f, _ := landStackSetup(t, cleanHighRound)
		var listed []string
		inner := ghPRListFn
		ghPRListFn = func(branch, state string) (string, error) {
			listed = append(listed, branch)
			return inner(branch, state)
		}
		execCmd(t, "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		if len(listed) != 1 || listed[0] != "demo/0001.low" {
			t.Errorf("pr list calls = %v, want only the lower layer's branch", listed)
		}
		if len(f.stackCalls) != 1 {
			t.Errorf("stack calls = %v", f.stackCalls)
		}
	})

	t.Run("merge failure names the layer and the next step", func(t *testing.T) {
		f, _ := landStackSetup(t, cleanHighRound)
		f.mergeErr = fmt.Errorf("boom")
		out := landStackRefusal(t, "--land-stack", "20")
		if !strings.Contains(out, "land_stack_merge_failed") || !strings.Contains(out, "fix_hint") || !strings.Contains(out, "boom") {
			t.Errorf("refusal = %s", out)
		}
	})

	t.Run("unreadable post-merge state keeps its error", func(t *testing.T) {
		f, _ := landStackSetup(t, cleanHighRound)
		f.mergeHook = func() { f.stateErr = true }
		out := landStackRefusal(t, "--land-stack", "20")
		if !strings.Contains(out, "land_stack_state_verify_failed") || !strings.Contains(out, "gh down") {
			t.Errorf("refusal = %s", out)
		}
	})
}

func TestLandStackRerun(t *testing.T) {
	t.Run("second run is a no-op with the same end state", func(t *testing.T) {
		f, vault := landStackSetup(t, cleanHighRound)
		execCmd(t, "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		execCmd(t, "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		if len(f.stackCalls) != 1 {
			t.Errorf("gh stack merge calls = %v, want one", f.stackCalls)
		}
		for _, slug := range []string{"0001.low", "0002.high"} {
			if got := issueStatus(t, vault, slug); got != "resolved" {
				t.Errorf("%s status = %v", slug, got)
			}
		}
	})

	t.Run("partial merge resolves both on re-run", func(t *testing.T) {
		f, vault := landStackSetup(t, cleanHighRound)
		f.state[10] = "MERGED"
		execCmd(t, "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20")
		if len(f.stackCalls) != 1 {
			t.Errorf("gh stack merge calls = %v, want one for the open layer", f.stackCalls)
		}
		for _, slug := range []string{"0001.low", "0002.high"} {
			if got := issueStatus(t, vault, slug); got != "resolved" {
				t.Errorf("%s status = %v", slug, got)
			}
		}
	})

	t.Run("json is one array and local-validated is audited", func(t *testing.T) {
		f, vault := landStackSetup(t, cleanHighRound)
		stdout, _, err := runCmd(t, newRootCmd(), "transition", "issue", "demo.0002.high", "resolved", "--land-stack", "20", "--local-validated", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var rs []transitionResult
		if err := json.Unmarshal([]byte(stdout), &rs); err != nil || len(rs) != 2 {
			t.Fatalf("stdout = %q, want one JSON array of 2 (err %v)", stdout, err)
		}
		if len(f.allChecks) != 0 {
			t.Errorf("CI gate ran under --local-validated: %v", f.allChecks)
		}
		a := loadIssueDoc(t, vault, "issue.demo.0002.high")
		if !strings.Contains(a.Body, "--local-validated: CI gate bypassed") {
			t.Errorf("audit line lacks the local-validated note:\n%s", a.Body)
		}
	})
}
