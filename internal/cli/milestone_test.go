package cli

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeMilestoneIssue writes an issue fixture linked to a milestone via the
// `milestone` slot, so the index emits a relation='milestone' link the
// done-signal aggregates.
func writeMilestoneIssue(t *testing.T, vault, id, status, milestone string) {
	t.Helper()
	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", id+".md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": id, "description": "fixture description",
			"created": "2026-01-01", "updated": "2026-01-01",
			"status": status, "project": "demo", "severity": "medium",
			"goal": "fixture goal is done", "milestone": "[[milestone." + milestone + "]]",
		},
		Body: "fixture body\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestMilestoneStatus_JSON_ReportsDoneSignal(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.m1", "open")
	writeMilestoneIssue(t, vault, "demo.a", "resolved", "demo.m1")
	writeMilestoneIssue(t, vault, "demo.b", "open", "demo.m1")
	execCmd(t, "reindex")

	out := execCmd(t, "milestone", "status", "demo.m1", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if got["milestone"] != "demo.m1" || got["resolved"] != float64(1) || got["total"] != float64(2) || got["done"] != false {
		t.Fatalf("not-done signal mismatch: %v", got)
	}

	// Resolve the open issue: the milestone is now done.
	writeMilestoneIssue(t, vault, "demo.b", "resolved", "demo.m1")
	execCmd(t, "reindex")
	out = execCmd(t, "milestone", "status", "demo.m1", "--json")
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if got["resolved"] != float64(2) || got["done"] != true {
		t.Fatalf("done signal mismatch: %v", got)
	}
}

func TestMilestoneStatus_UnknownMilestone_Errors(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"milestone", "status", "demo.nope", "--json"})
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("unknown milestone should error; got output: %s", buf.String())
	}
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("err = %v, want ErrArtifactNotFound", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "demo.nope") {
		t.Errorf("error message %q does not name the missing id", msg)
	}
}
