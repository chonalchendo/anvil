package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
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

	// ...and its per-task rows are queryable as the canonical list envelope with
	// the telemetry columns present.
	var env2 struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	tasksOut := execCmd(t, "build", "tasks", env.RunID, "--json")
	if err := json.Unmarshal([]byte(tasksOut), &env2); err != nil {
		t.Fatalf("tasks output not JSON: %v\n%s", err, tasksOut)
	}
	if env2.Total != 1 || len(env2.Items) != 1 {
		t.Fatalf("got total=%d items=%d, want 1:\n%s", env2.Total, len(env2.Items), tasksOut)
	}
	r := env2.Items[0]
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

func TestReviewPhase_EmitsReviewTaskOnlyForVerifiedComplete(t *testing.T) {
	// The complete wave: foo passed the advance-gate (a PR exists), bar did not.
	completeTasks := []core.Task{
		{ID: "demo.foo", Cwd: "/wt/foo", Branch: "demo/foo"},
		{ID: "demo.bar", Cwd: "/wt/bar", Branch: "demo/bar"},
	}
	sum := &build.Summary{Outcomes: map[string]build.TaskOutcome{
		"demo.foo": {TaskID: "demo.foo", Outcome: "success"},
		"demo.bar": {TaskID: "demo.bar", Outcome: "failed"},
	}}

	reviews := reviewTasksFromTasks(completeTasks, sum)

	// Only the verified-complete task gets a review task.
	if len(reviews) != 1 {
		t.Fatalf("got %d review tasks, want 1 (only the success)", len(reviews))
	}
	r := reviews[0]
	if r.ID != "demo.foo" {
		t.Errorf("review task ID = %q, want demo.foo", r.ID)
	}
	if len(r.SkillsToLoad) != 1 || r.SkillsToLoad[0] != "reviewing-pr" {
		t.Errorf("review SkillsToLoad = %v, want [reviewing-pr]", r.SkillsToLoad)
	}
	// The phases decouple through gh state: the review body points the skill at
	// the branch, and the task reuses the issue's worktree so gh resolves the PR.
	if r.Cwd != "/wt/foo" || r.Branch != "demo/foo" {
		t.Errorf("review Cwd/Branch = %q/%q, want /wt/foo/demo/foo", r.Cwd, r.Branch)
	}
	for _, want := range []string{"demo.foo", "reviewing-pr skill", "gh pr list --head demo/foo"} {
		if !strings.Contains(r.Body, want) {
			t.Errorf("review body missing %q; got:\n%s", want, r.Body)
		}
	}
}

func TestReviewPhase_TelemetryTagsEachPhaseRow(t *testing.T) {
	db, err := index.Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer db.Close() //nolint:errcheck // test cleanup

	phases := []phaseSummary{
		{phase: "complete", sum: &build.Summary{Outcomes: map[string]build.TaskOutcome{
			"demo.foo": {TaskID: "demo.foo", Model: "claude-sonnet-4-6", Outcome: "success"},
		}}},
		{phase: "review", sum: &build.Summary{Outcomes: map[string]build.TaskOutcome{
			"demo.foo": {TaskID: "demo.foo", Model: "claude-sonnet-4-6", Outcome: "success"},
		}}},
	}
	if err := recordBuildTelemetry(db, "run-1", "2026-06-21T00:00:00Z", "demo", "", false, phases); err != nil {
		t.Fatalf("record telemetry: %v", err)
	}

	rows, err := db.BuildTasksByRun("run-1")
	if err != nil {
		t.Fatalf("query telemetry: %v", err)
	}
	// Same task_id appears once per phase — a distinct row each, no PK collision.
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (one complete, one review for demo.foo)", len(rows))
	}
	got := map[string]bool{}
	for _, r := range rows {
		if r.TaskID != "demo.foo" {
			t.Errorf("row task_id = %q, want demo.foo", r.TaskID)
		}
		got[r.Phase] = true
	}
	for _, phase := range []string{"complete", "review"} {
		if !got[phase] {
			t.Errorf("no telemetry row tagged phase %q; rows=%+v", phase, rows)
		}
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
