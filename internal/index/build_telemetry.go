package index

import "fmt"

// BuildRun is one `anvil build` invocation's run-level record. DryRun marks a
// plan-only run (no agent spawned); its task rows carry zero token/cost columns.
type BuildRun struct {
	RunID     string `json:"run_id"`
	StartedAt string `json:"started_at"`
	Project   string `json:"project"`
	Milestone string `json:"milestone"`
	DryRun    bool   `json:"dry_run"`
	Tasks     int    `json:"tasks"`
}

// BuildTask is one task's telemetry within a build run. The engine's in-memory
// TaskOutcome is the source shape; the driver projects it onto this row.
type BuildTask struct {
	RunID       string  `json:"run_id"`
	TaskID      string  `json:"task_id"`
	Wave        int     `json:"wave"`
	Model       string  `json:"model"`
	Effort      string  `json:"effort"`
	Outcome     string  `json:"outcome"`
	TokensIn    int64   `json:"tokens_in"`
	TokensOut   int64   `json:"tokens_out"`
	CacheRead   int64   `json:"cache_read"`
	CacheWrite  int64   `json:"cache_write"`
	CostUSD     float64 `json:"cost_usd"`
	DurationMS  int64   `json:"duration_ms"`
	AgentTimeMS int64   `json:"agent_time_ms"`
	VerifyExit  int     `json:"verify_exit"`
}

// InsertBuildRun records one run row. Rows are append-only; each build run is a
// distinct record keyed by run id.
func (d *DB) InsertBuildRun(r BuildRun) error {
	dryRun := 0
	if r.DryRun {
		dryRun = 1
	}
	const q = `INSERT INTO build_runs(run_id, started_at, project, milestone, dry_run, tasks)
VALUES(?, ?, ?, ?, ?, ?)`
	if _, err := d.sql.Exec(q, r.RunID, r.StartedAt, r.Project, r.Milestone, dryRun, r.Tasks); err != nil {
		return fmt.Errorf("insert build run %s: %w", r.RunID, err)
	}
	return nil
}

// InsertBuildTasks records all of a run's per-task rows in one transaction so a
// run's telemetry lands atomically.
func (d *DB) InsertBuildTasks(tasks []BuildTask) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin build tasks tx: %w", err)
	}
	const q = `INSERT INTO build_tasks(run_id, task_id, wave, model, effort, outcome,
tokens_in, tokens_out, cache_read, cache_write, cost_usd, duration_ms, agent_time_ms, verify_exit)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, t := range tasks {
		if _, err := tx.Exec(q, t.RunID, t.TaskID, t.Wave, t.Model, t.Effort, t.Outcome,
			t.TokensIn, t.TokensOut, t.CacheRead, t.CacheWrite, t.CostUSD,
			t.DurationMS, t.AgentTimeMS, t.VerifyExit); err != nil {
			tx.Rollback() //nolint:errcheck,gosec // rollback before returning the insert error
			return fmt.Errorf("insert build task %s/%s: %w", t.RunID, t.TaskID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit build tasks tx: %w", err)
	}
	return nil
}

// BuildTasksByRun returns a run's per-task telemetry in wave then task-id order.
func (d *DB) BuildTasksByRun(runID string) ([]BuildTask, error) {
	const q = `SELECT run_id, task_id, wave, model, effort, outcome,
tokens_in, tokens_out, cache_read, cache_write, cost_usd, duration_ms, agent_time_ms, verify_exit
FROM build_tasks WHERE run_id = ? ORDER BY wave, task_id`
	rs, err := d.sql.Query(q, runID)
	if err != nil {
		return nil, fmt.Errorf("build tasks for %s: %w", runID, err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	out := []BuildTask{}
	for rs.Next() {
		var t BuildTask
		if err := rs.Scan(&t.RunID, &t.TaskID, &t.Wave, &t.Model, &t.Effort, &t.Outcome,
			&t.TokensIn, &t.TokensOut, &t.CacheRead, &t.CacheWrite, &t.CostUSD,
			&t.DurationMS, &t.AgentTimeMS, &t.VerifyExit); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rs.Err()
}
