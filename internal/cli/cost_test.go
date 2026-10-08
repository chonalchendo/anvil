package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

const costID = "issue.anvil.0001.cost"

func writeCostIssue(t *testing.T, vault string, links []any, session string) {
	t.Helper()
	fm := map[string]any{
		"type": "issue", "title": costID, "description": "fixture",
		"created": "2026-05-15", "updated": "2026-05-15",
		"status": "in-progress", "project": "anvil", "severity": "medium",
		"tags": []any{"domain/cli"},
	}
	if links != nil {
		fm["external_links"] = links
	}
	if session != "" {
		fm["claim_session"] = session
	}
	a := &core.Artifact{
		Path:        filepath.Join(vault, "70-issues", costID+".md"),
		FrontMatter: fm,
		Body: "## Review findings — PR 7, round 1 @ a\n\nnone\n\n## Review findings — PR 7, round 2 @ b\n\nnone\n\n" +
			"## Review findings — PR 3, round 1 @ c\n\nother PR\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func stubCostEnv(t *testing.T, view func(int, string) ([]byte, error)) {
	t.Helper()
	prevView, prevTop := ghPRViewJSONFn, gitToplevelFn
	t.Cleanup(func() { ghPRViewJSONFn, gitToplevelFn = prevView, prevTop })
	ghPRViewJSONFn = view
	gitToplevelFn = func() (string, error) { return "/Users/x/anvil", nil }
}

func okView(_ int, _ string) ([]byte, error) {
	return []byte(`{"additions":30,"deletions":12,"changedFiles":4}`), nil
}

func asst(id string, in, cc, cr, out int) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "attributionAgent": "anvil-issue-worker",
		"message": map[string]any{"id": id, "usage": map[string]int{
			"input_tokens": in, "cache_creation_input_tokens": cc, "cache_read_input_tokens": cr, "output_tokens": out,
		}},
	})
	return string(b)
}

func writeTranscript(t *testing.T, dir, name string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCostDerivesRoundsDiffAndDedupedTokens(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	writeCostIssue(t, vault, []any{"https://github.com/o/r/pull/3", "https://github.com/o/r/pull/7"}, "sess1")
	var gotNum int
	stubCostEnv(t, func(n int, f string) ([]byte, error) {
		gotNum = n
		if f != "additions,deletions,changedFiles" {
			t.Errorf("fields = %q", f)
		}
		return okView(n, f)
	})
	projects := t.TempDir()
	dir := filepath.Join(projects, "-Users-x-anvil", "sess1", "subagents")
	writeTranscript(t, dir, "agent-a.jsonl",
		`{"type":"user","message":{"role":"user","content":"Complete anvil issue `+costID+`."}}`,
		asst("m1", 100, 200, 300, 400), asst("m1", 100, 200, 300, 400), asst("m2", 1, 2, 3, 4))
	writeTranscript(t, dir, "agent-b.jsonl",
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Review `+costID+`"}]}}`,
		`{"type":"assistant","message":{"id":"r1","usage":{"input_tokens":5}}}`)
	writeTranscript(t, dir, "agent-c.jsonl",
		`{"type":"user","message":{"role":"user","content":"Complete anvil issue issue.anvil.0002.other"}}`,
		asst("m9", 5000, 0, 0, 0))

	out, _, err := runCmd(t, newCostCmd(), costID, "--projects-dir", projects, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rec costRecord
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	want := costRecord{
		Issue: costID, PR: 7, Rounds: 2, Additions: 30, Deletions: 12, Diff: 42, Files: 4, Tokens: 1015,
		ByAgent: map[string]int{"anvil-issue-worker": 1010, "subagent-unknown": 5}, Session: "sess1",
	}
	if d := cmp.Diff(want, rec, cmp.AllowUnexported(costRecord{})); d != "" {
		t.Errorf("record mismatch (-want +got):\n%s", d)
	}
	if gotNum != 7 {
		t.Errorf("viewed PR %d, want 7", gotNum)
	}
}

func TestCostNoPRRefuses(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	writeCostIssue(t, vault, nil, "")
	stubCostEnv(t, okView)
	_, _, err := runCmd(t, newCostCmd(), costID)
	var se *errfmt.Structured
	if !errors.As(err, &se) || se.Code != "cost_no_pr" {
		t.Fatalf("err = %v, want cost_no_pr", err)
	}
}

func TestCostPRViewFailureRefuses(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	writeCostIssue(t, vault, []any{"https://github.com/o/r/pull/7"}, "")
	stubCostEnv(t, func(int, string) ([]byte, error) { return nil, errors.New("boom") })
	_, _, err := runCmd(t, newCostCmd(), costID)
	var se *errfmt.Structured
	if !errors.As(err, &se) || se.Code != "cost_pr_view_failed" {
		t.Fatalf("err = %v, want cost_pr_view_failed", err)
	}
}

func TestCostMissingTranscriptsGiveZeroTokensAndNotice(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	writeCostIssue(t, vault, []any{"https://github.com/o/r/pull/7"}, "gone")
	stubCostEnv(t, okView)
	projects := t.TempDir()
	out, errOut, err := runCmd(t, newCostCmd(), costID, "--projects-dir", projects, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rec costRecord
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Tokens != 0 || len(rec.ByAgent) != 0 || rec.Rounds != 2 {
		t.Errorf("record = %+v", rec)
	}
	if !strings.Contains(errOut, filepath.Join(projects, "-Users-x-anvil", "gone", "subagents")) {
		t.Errorf("notice %q does not name the path looked at", errOut)
	}
}

func TestCostTextOutput(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	writeCostIssue(t, vault, []any{"https://github.com/o/r/pull/7"}, "")
	stubCostEnv(t, okView)
	out, _, err := runCmd(t, newCostCmd(), costID, "--projects-dir", t.TempDir())
	if err != nil || !strings.Contains(out, "rounds:  2") || !strings.Contains(out, "diff:    42") {
		t.Fatalf("out = %q err = %v", out, err)
	}
}
