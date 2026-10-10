package cli

import (
	"strings"
	"testing"
)

func TestEscapedCountsReopenedAndFixedIssues(t *testing.T) {
	one, zero := 1, 0
	rows := []milestoneIssueRow{
		{ID: "issue.p.0001.a", Status: "resolved", issueOutcome: issueOutcome{Reopens: &one}},
		{ID: "issue.p.0002.b", Status: "resolved", issueOutcome: issueOutcome{Reopens: &zero}},
		{ID: "issue.p.0003.c", Status: "resolved"},
		{ID: "issue.p.0004.d", Status: "abandoned", issueOutcome: issueOutcome{Reopens: &one}},
	}
	fixed := map[string]bool{"issue.p.0001.a": true, "issue.p.0003.c": true, "issue.p.0002.b": true}
	o := sumOutcome(rows, nil, fixed)
	if o.Escaped != 3 {
		t.Fatalf("Escaped = %d, want 3", o.Escaped)
	}
	if !strings.HasSuffix(o.line(), "3 escaped") {
		t.Fatalf("line = %q", o.line())
	}
}

func TestEscapedFromMilestoneStatusCountsOnlyFixesTargets(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.m1", "open")
	writeMilestoneIssue(t, vault, "demo.rel", "resolved", "demo.m1")
	writeMilestoneIssue(t, vault, "demo.esc", "resolved", "demo.m1")
	writeMilestoneIssue(t, vault, "demo.open", "open", "demo.m1")
	writeMilestoneIssue(t, vault, "demo.src", "resolved", "demo.other")
	execCmd(t, "link", "issue", "demo.src", "issue", "demo.rel", "--relation", "related")
	execCmd(t, "link", "issue", "demo.src", "issue", "demo.esc", "--relation", "fixes")
	execCmd(t, "link", "issue", "demo.src", "issue", "demo.open", "--relation", "fixes")
	execCmd(t, "reindex")

	out := execCmdJSON(t, "milestone", "status", "demo.m1", "--json")
	var got struct {
		Outcome struct {
			Escaped int `json:"escaped"`
		} `json:"outcome"`
	}
	if err := jsonUnmarshal(t, strings.TrimSpace(out), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if got.Outcome.Escaped != 1 {
		t.Fatalf("outcome.escaped = %d, want 1 (related and unresolved fixes targets excluded)\n%s", got.Outcome.Escaped, out)
	}
}
