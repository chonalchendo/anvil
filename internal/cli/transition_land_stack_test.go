package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// stackFake answers gh for a two-layer stack: PR 10 (demo/0001.low, base
// master) under PR 20 (demo/0002.high, base demo/0001.low).
type stackFake struct {
	*sideFXStub
	merged     bool
	stackCalls []int
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

	f := &stackFake{sideFXStub: stubSideFX(t)}
	f.homeDir = t.TempDir()
	refs := map[int]struct{ head, base string }{
		10: {"demo/0001.low", "master"},
		20: {"demo/0002.high", "demo/0001.low"},
	}
	ghPRViewJSONFn = func(num int, fields string) ([]byte, error) {
		r := refs[num]
		vals := map[string]string{
			"state": "OPEN", "mergeable": "MERGEABLE", "mergeStateStatus": "CLEAN",
			"headRefOid": landTestHead, "headRefName": r.head, "baseRefName": r.base,
		}
		if f.merged {
			vals["state"] = "MERGED"
		}
		var parts []string
		for _, k := range strings.Split(fields, ",") {
			parts = append(parts, fmt.Sprintf("%q:%q", k, vals[k]))
		}
		return []byte("{" + strings.Join(parts, ",") + "}"), nil
	}
	prevList := ghPRListFn
	ghPRListFn = func(branch string) (string, error) {
		if branch == "demo/0001.low" {
			return "https://github.com/o/r/pull/10", nil
		}
		return "", nil
	}
	prevStack := ghStackMergeFn
	ghStackMergeFn = func(num int) error {
		f.stackCalls = append(f.stackCalls, num)
		f.merged = true
		return nil
	}
	t.Cleanup(func() { ghPRListFn, ghStackMergeFn = prevList, prevStack })
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
