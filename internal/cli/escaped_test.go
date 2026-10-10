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
	fixed := map[string]bool{"issue.p.0003.c": true, "issue.p.0002.b": true}
	o := sumOutcome(rows, nil, fixed)
	if o.Escaped != 3 {
		t.Fatalf("Escaped = %d, want 3", o.Escaped)
	}
	if !strings.HasSuffix(o.line(), "3 escaped") {
		t.Fatalf("line = %q", o.line())
	}
}
