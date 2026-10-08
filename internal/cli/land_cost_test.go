package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

const landCostID = "issue.demo.0001.foo"

// landCostSetup claims a fixture whose id the transcript scan can match, with
// two review rounds on PR 42, then returns the stub and the vault.
func landCostSetup(t *testing.T) (*sideFXStub, string) {
	t.Helper()
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess1")
	execCmd(t, "init", vault)
	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", landCostID+".md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "foo", "description": "fixture description",
			"created": "2026-01-01", "updated": "2026-01-01",
			"status": "open", "project": "demo", "severity": "medium",
			"tags": []any{"domain/dev-tools"}, "goal": "fixture goal is done",
		},
		Body: fixtureIssueBody + "\n## Review findings — PR 42, round 2 @ 0123456\n\nnone\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")
	execCmd(t, "transition", "issue", landCostID, "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.homeDir = t.TempDir()
	writeTranscript(t, filepath.Join(s.homeDir, ".claude", "projects", "p", "sess1", "subagents"), "agent-a.jsonl",
		`{"type":"user","message":{"role":"user","content":"Complete anvil issue `+landCostID+`."}}`,
		asst("m1", 1000, 0, 0, 5), asst("m1", 1000, 0, 0, 10))
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField[landHeadFields] = headJSON(landTestHead, "demo/0001.foo")
	s.listEntries["demo/0001.foo"] = worktreeInfo{path: "/worktrees/foo"}
	return s, vault
}

func TestLandPRCostStampsFourFields(t *testing.T) {
	s, vault := landCostSetup(t)
	s.viewByField["additions,deletions,changedFiles"] = []byte(`{"additions":100,"deletions":53,"changedFiles":2}`)
	s.viewByField["url"] = []byte(`{"url":"https://github.com/o/r/pull/42"}`)

	execCmd(t, "transition", "issue", landCostID, "resolved", "--land-pr", "42")

	a := loadIssueDoc(t, vault, landCostID)
	for k, want := range map[string]int{"cost_rounds": 2, "cost_diff": 153, "cost_files": 2, "cost_tokens": 1010} {
		if got, ok := a.FrontMatter[k].(int); !ok || got != want {
			t.Errorf("%s = %#v, want %d", k, a.FrontMatter[k], want)
		}
	}
	links, _ := a.FrontMatter["external_links"].([]any)
	if len(links) != 1 || links[0] != "https://github.com/o/r/pull/42" {
		t.Errorf("external_links = %v", links)
	}
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
}

func TestLandPRCostViewFailureWarnsAndResolves(t *testing.T) {
	s, vault := landCostSetup(t)
	s.viewByFieldE["additions,deletions,changedFiles"] = errors.New("boom")

	_, errOut, err := runCmd(t, newRootCmd(), "transition", "issue", landCostID, "resolved", "--land-pr", "42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut, "warning: land-pr 42: cost not stamped") {
		t.Errorf("stderr = %q, want the cost warning", errOut)
	}
	a := loadIssueDoc(t, vault, landCostID)
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", a.FrontMatter["status"])
	}
	for _, k := range []string{"cost_rounds", "cost_diff", "cost_files", "cost_tokens"} {
		if _, ok := a.FrontMatter[k]; ok {
			t.Errorf("%s set despite the cost error", k)
		}
	}
}

func TestLandPRCostSkipsURLFetchWhenLinked(t *testing.T) {
	s, vault := landCostSetup(t)
	s.viewByField["additions,deletions,changedFiles"] = []byte(`{"additions":100,"deletions":53,"changedFiles":2}`)
	execCmd(t, "link", "issue", landCostID, "--external", "https://github.com/o/r/pull/42")

	execCmd(t, "transition", "issue", landCostID, "resolved", "--land-pr", "42")

	for _, f := range s.viewCalls {
		if f == "url" {
			t.Errorf("url view fetched although the pr is linked: %v", s.viewCalls)
		}
	}
	a := loadIssueDoc(t, vault, landCostID)
	if links, _ := a.FrontMatter["external_links"].([]any); len(links) != 1 {
		t.Errorf("external_links = %v, want 1 entry", links)
	}
	if a.FrontMatter["cost_rounds"] != 2 {
		t.Errorf("cost_rounds = %v, want 2", a.FrontMatter["cost_rounds"])
	}
}

func TestLandPRCostURLFetchFailureStillStamps(t *testing.T) {
	s, vault := landCostSetup(t)
	s.viewByField["additions,deletions,changedFiles"] = []byte(`{"additions":100,"deletions":53,"changedFiles":2}`)
	s.viewByFieldE["url"] = errors.New("boom")

	_, errOut, err := runCmd(t, newRootCmd(), "transition", "issue", landCostID, "resolved", "--land-pr", "42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut, "warning: land-pr 42: pr url not linked") {
		t.Errorf("stderr = %q, want the url warning", errOut)
	}
	a := loadIssueDoc(t, vault, landCostID)
	if a.FrontMatter["cost_tokens"] != 1010 {
		t.Errorf("cost_tokens = %v, want 1010", a.FrontMatter["cost_tokens"])
	}
}
