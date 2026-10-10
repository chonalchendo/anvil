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
	runVerify(t, vault, id)
	if v := load()["outcome_first_verdict"]; v != nil {
		t.Fatalf("first verdict set before claim: %v", v)
	}

	edit(func(a *core.Artifact) { stampIssueGate(a, "in-progress", "", time.Now()) })
	runVerify(t, vault, id)
	edit(func(a *core.Artifact) {
		a.Body = strings.Replace(a.Body, "false", "true", 1)
		stampIssueGate(a, "in-progress", "", time.Now())
	})
	runVerify(t, vault, id)
	fm := load()
	if fm["verified_verdict"] != "pass" || fm["outcome_first_verdict"] != "fail" {
		t.Fatalf("verdict=%v first=%v, want pass/fail", fm["verified_verdict"], fm["outcome_first_verdict"])
	}

	// Escalation: counted, and the count survives leaving escalated.
	edit(func(a *core.Artifact) {
		stampIssueGate(a, "escalated", "blocked", time.Now())
		stampIssueGate(a, "open", "", time.Now())
		stampIssueGate(a, "escalated", "blocked again", time.Now())
	})
	if n := load()["outcome_escalations"]; n != 2 {
		t.Fatalf("outcome_escalations = %v, want 2", n)
	}

	// Reopen: only a reverse move counts; escalated -> open does not.
	edit(func(a *core.Artifact) { a.FrontMatter["status"] = "resolved" })
	execCmd(t, "reindex")
	execCmd(t, "transition", "issue", id, "open", "--reason", "regressed")
	if n := load()["outcome_reopens"]; n != 1 {
		t.Fatalf("outcome_reopens = %v, want 1", n)
	}

	// Re-scope: counted only when --accept-change changes the lock.
	runVerify(t, vault, id)
	if load()["outcome_rescopes"] != nil {
		t.Fatal("rescope counted without a lock change")
	}
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
