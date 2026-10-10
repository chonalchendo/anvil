package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestOutcomeIssue(t *testing.T) {
	const id = "issue.anvil.0384.outcome"
	vault := setupVault(t)
	writeVerifyIssue(t, vault, id, "false", "true")
	path := filepath.Join(vault, "70-issues", id+".md")
	edit := func(f func(a *core.Artifact)) {
		t.Helper()
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		f(a)
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	load := func() map[string]any {
		t.Helper()
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		return a.FrontMatter
	}

	if fm := load(); fm["outcome_escalations"] != nil || fm["outcome_reopens"] != nil {
		t.Fatalf("counters written on create: %v", fm)
	}

	// A verdict before the first claim is not the first verdict.
	_, _, _ = runVerify(t, vault, id) // red verdict returns an error; the record is what matters
	if v := load()["outcome_first_verdict"]; v != nil {
		t.Fatalf("first verdict set before claim: %v", v)
	}

	edit(func(a *core.Artifact) { stampIssueGate(a, "in-progress", "", time.Now()) })
	_, _, _ = runVerify(t, vault, id) // red verdict returns an error; the record is what matters
	edit(func(a *core.Artifact) {
		a.Body = strings.Replace(a.Body, "false", "true", 1)
		stampIssueGate(a, "in-progress", "", time.Now())
	})
	if _, _, err := runVerify(t, vault, id); err != nil {
		t.Fatal(err)
	}
	fm := load()
	if fm["verified_verdict"] != "pass" || fm["outcome_first_verdict"] != "fail" {
		t.Fatalf("verdict=%v first=%v, want pass/fail", fm["verified_verdict"], fm["outcome_first_verdict"])
	}

	// Re-scope: an --accept-change with no lock change is not a re-scope.
	t.Setenv("ANVIL_VAULT", vault)
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--accept-change", "--json"); err != nil {
		t.Fatal(err)
	}
	if load()["outcome_rescopes"] != nil {
		t.Fatal("rescope counted without a lock change")
	}

	// Escalation: counted, and the count survives leaving escalated.
	edit(func(a *core.Artifact) {
		stampIssueGate(a, "escalated", "blocked", time.Now())
		stampIssueGate(a, "open", "", time.Now())
		stampIssueGate(a, "escalated", "blocked again", time.Now())
		a.FrontMatter["status"] = "escalated"
	})
	if n := load()["outcome_escalations"]; n != 2 {
		t.Fatalf("outcome_escalations = %v, want 2", n)
	}

	// Reopen: only a reverse move counts; escalated -> open does not.
	execCmd(t, "reindex")
	execCmd(t, "transition", "issue", id, "open", "--reason", "unblocked")
	if n := load()["outcome_reopens"]; n != nil {
		t.Fatalf("outcome_reopens = %v after escalated -> open, want unset", n)
	}
	edit(func(a *core.Artifact) { a.FrontMatter["status"] = "resolved" })
	execCmd(t, "reindex")
	execCmd(t, "transition", "issue", id, "open", "--reason", "regressed")
	if n := load()["outcome_reopens"]; n != 1 {
		t.Fatalf("outcome_reopens = %v, want 1", n)
	}

	// Re-scope: counted when --accept-change changes the lock.
	edit(func(a *core.Artifact) {
		a.Body = strings.Replace(a.Body, "```bash\ntrue\n```\n\n### Indirect", "```bash\ntrue; true\n```\n\n### Indirect", 1)
	})
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--accept-change", "--json"); err != nil {
		t.Fatal(err)
	}
	if n := load()["outcome_rescopes"]; n != 1 {
		t.Fatalf("outcome_rescopes = %v, want 1", n)
	}
}

func TestOutcomeMilestone(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.m1", "in-progress")
	mpath := filepath.Join(vault, "85-milestones", "demo.m1.md")
	setStatus := func(status string) {
		t.Helper()
		a, err := core.LoadArtifact(mpath)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["status"] = status
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
		execCmd(t, "reindex")
	}
	count := func(key string) any {
		t.Helper()
		a, err := core.LoadArtifact(mpath)
		if err != nil {
			t.Fatal(err)
		}
		return a.FrontMatter[key]
	}

	execCmd(t, "reindex")
	execCmd(t, "transition", "milestone", "demo.m1", "planned")
	setStatus("in-progress")
	execCmd(t, "transition", "milestone", "demo.m1", "planned")
	if n := count("outcome_amendments"); n != 2 {
		t.Fatalf("outcome_amendments = %v, want 2", n)
	}
	if n := count("outcome_reopens"); n != nil {
		t.Fatalf("outcome_reopens = %v before any reopen, want unset", n)
	}
	setStatus("done")
	execCmd(t, "transition", "milestone", "demo.m1", "in-progress", "--reason", "regressed")
	if n := count("outcome_reopens"); n != 1 {
		t.Fatalf("outcome_reopens = %v, want 1", n)
	}
	if n := count("outcome_amendments"); n != 2 {
		t.Fatalf("outcome_amendments = %v after reopen, want 2", n)
	}

	seed := func(id, status string, fm map[string]any) {
		t.Helper()
		writeMilestoneIssue(t, vault, id, status, "demo.m1")
		p := filepath.Join(vault, "70-issues", id+".md")
		a, err := core.LoadArtifact(p)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range fm {
			a.FrontMatter[k] = v
		}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	const pr1, pr2 = "https://github.com/o/r/pull/1", "https://github.com/o/r/pull/2"
	seed("demo.a", "resolved", map[string]any{"outcome_first_verdict": "pass", "external_links": []any{pr1}})
	seed("demo.b", "resolved", map[string]any{"outcome_first_verdict": "fail", "outcome_rescopes": 1, "outcome_reopens": 1, "external_links": []any{pr1}})
	seed("demo.c", "resolved", map[string]any{"outcome_escalations": 2, "external_links": []any{pr1, pr2}})
	seed("demo.d", "abandoned", map[string]any{"outcome_escalations": 5, "outcome_first_verdict": "pass"})
	execCmd(t, "reindex")

	var got struct {
		Outcome milestoneOutcome
		Issues  []struct {
			ID                  string
			OutcomeEscalations  *int    `json:"outcome_escalations"`
			OutcomeFirstVerdict *string `json:"outcome_first_verdict"`
		}
	}
	out := execCmdJSON(t, "milestone", "status", "demo.m1", "--json")
	if err := jsonUnmarshal(t, strings.TrimSpace(out), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	want := milestoneOutcome{Issues: 3, OnePRNoRescope: 1, FirstPass: 1, Escalations: 2, Reopens: 1, Rescopes: 1, Amendments: 2}
	if got.Outcome != want {
		t.Fatalf("outcome = %+v, want %+v", got.Outcome, want)
	}
	if len(got.Issues) != 4 || got.Issues[0].OutcomeFirstVerdict == nil || got.Issues[1].OutcomeEscalations != nil {
		t.Fatalf("issue rows mismatch: %+v", got.Issues)
	}
	text := execCmdJSON(t, "milestone", "status", "demo.m1")
	if !strings.Contains(text, "Outcome: 1/3 one-PR-no-rescope, 1 first-pass, 2 escalations, 1 reopens, 1 rescopes, 2 amendments\n") {
		t.Fatalf("text missing Outcome line:\n%s", text)
	}
}
