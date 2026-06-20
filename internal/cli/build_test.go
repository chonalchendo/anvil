package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuild_DryRunJSON_EmitsPlanEnvelope(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t) // demo.foo: open, unblocked → ready

	out := execCmd(t, "build", "--dry-run", "--json", "--project", "demo")
	// --dry-run --json emits a single plan envelope so callers can assert
	// per-task fields (config_dir uniqueness, auto_merge) and the run id without
	// slurp mode.
	for _, want := range []string{`"run_id":`, `"task_id":"demo.foo"`, `"config_dir":`, `"auto_merge":false`, `"tasks":`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestBuild_DryRun_PersistsQueryableTelemetry(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t) // demo.foo: open, unblocked → ready

	// A dry-run records a run keyed by run id...
	var env struct {
		RunID string `json:"run_id"`
	}
	out := execCmd(t, "build", "--dry-run", "--json", "--project", "demo")
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("plan envelope not JSON: %v\n%s", err, out)
	}
	if env.RunID == "" {
		t.Fatalf("plan envelope carried no run_id:\n%s", out)
	}

	// ...and its per-task rows are queryable with the telemetry columns present.
	var rows []map[string]any
	tasksOut := execCmd(t, "build", "tasks", env.RunID, "--json")
	if err := json.Unmarshal([]byte(tasksOut), &rows); err != nil {
		t.Fatalf("tasks output not JSON: %v\n%s", err, tasksOut)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d task rows, want 1:\n%s", len(rows), tasksOut)
	}
	r := rows[0]
	if r["task_id"] != "demo.foo" {
		t.Errorf("task_id = %v, want demo.foo", r["task_id"])
	}
	if r["outcome"] != "skipped_dry_run" {
		t.Errorf("outcome = %v, want skipped_dry_run", r["outcome"])
	}
	for _, col := range []string{"model", "tokens_in", "tokens_out", "cost_usd", "verify_exit"} {
		if _, ok := r[col]; !ok {
			t.Errorf("telemetry column %q absent from row:\n%s", col, tasksOut)
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

func TestBuild_DoneMilestone_ShortCircuits(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.m1", "open")
	writeMilestoneIssue(t, vault, "demo.a", "resolved", "demo.m1")
	execCmd(t, "reindex")

	// Every linked issue is resolved → the build loop's exit predicate fires
	// before selecting work, reporting the milestone done rather than the
	// generic no-ready notice.
	out := execCmd(t, "build", "--dry-run", "--project", "demo", "--milestone", "demo.m1")
	if !strings.Contains(out, "milestone demo.m1 is done (1/1 resolved)") {
		t.Errorf("expected done short-circuit; got:\n%s", out)
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
