package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuild_DryRunJSON_EmitsReadyIssueRecords(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t) // demo.foo: open, unblocked → ready

	out := execCmd(t, "build", "--dry-run", "--json", "--project", "demo")
	for _, want := range []string{`"task_id":"demo.foo"`, `"status":"skipped_dry_run"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestBuild_DryRunText_ListsReadyIssueIDs(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	out := execCmd(t, "build", "--dry-run", "--project", "demo")
	if !strings.Contains(out, "demo.foo") {
		t.Errorf("dry-run text output missing ready issue id demo.foo:\n%s", out)
	}
}

func TestBuild_NoReadyIssues_PrintsNotice(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	out := execCmd(t, "build", "--dry-run")
	if !strings.Contains(out, "no ready issues to dispatch") {
		t.Errorf("expected no-ready notice; got:\n%s", out)
	}
}

func TestBuild_MilestoneFilter_ExcludesUnmatchedIssue(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t) // demo.foo carries no milestone

	// A milestone filter matching nothing must exclude the unmatched issue
	// rather than dispatch it — yielding the no-ready notice.
	out := execCmd(t, "build", "--dry-run", "--project", "demo", "--milestone", "demo.nonexistent")
	if !strings.Contains(out, "no ready issues to dispatch") {
		t.Errorf("milestone filter should exclude demo.foo (no milestone); got:\n%s", out)
	}
}

func TestBuild_RejectsPositionalArgs(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"build", "some-plan-id", "--dry-run"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Errorf("build takes no positional args; want error, got nil\noutput: %s", buf.String())
	}
}

func TestReadyUnitsToTasks_MapsIDSkillAndStartContext(t *testing.T) {
	units := []readyUnit{
		{ID: "demo.a", Goal: "ship a", Severity: "high", Milestone: "demo.m1", Contracts: []string{"demo.c1"}, Path: "/v/demo.a.md"},
		{ID: "demo.b", Goal: "ship b", Severity: "low", Path: "/v/demo.b.md"},
	}
	tasks := readyUnitsToTasks(units)
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].ID != "demo.a" {
		t.Errorf("task[0].ID = %q, want demo.a", tasks[0].ID)
	}
	if len(tasks[0].SkillsToLoad) != 1 || tasks[0].SkillsToLoad[0] != "completing-issue" {
		t.Errorf("task[0].SkillsToLoad = %v, want [completing-issue]", tasks[0].SkillsToLoad)
	}
	// The body carries the assembled start-context, not just the id.
	for _, want := range []string{"demo.a", "Goal: ship a", "Severity: high", "Milestone: demo.m1", "Governing contracts: demo.c1", "Issue path: /v/demo.a.md"} {
		if !strings.Contains(tasks[0].Body, want) {
			t.Errorf("task[0].Body missing %q; got:\n%s", want, tasks[0].Body)
		}
	}
	// Empty milestone/contracts produce no blank scaffolding lines.
	if strings.Contains(tasks[1].Body, "Milestone:") || strings.Contains(tasks[1].Body, "Governing contracts:") {
		t.Errorf("task[1].Body should omit empty milestone/contracts; got:\n%s", tasks[1].Body)
	}
}
