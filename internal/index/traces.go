package index

import (
	"fmt"
	"time"
)

// Trace is one recorded build-task outcome, carrying the assembled prompt and
// the outcome string from the adapter run. Used by `anvil export traces` to
// emit an eval/RL dataset (prompt + outcome JSONL).
type Trace struct {
	ID         int64   `json:"id"`
	TaskID     string  `json:"task_id"`
	Prompt     string  `json:"prompt"`
	Outcome    string  `json:"outcome"`
	Model      string  `json:"model"`
	Effort     string  `json:"effort"`
	DurationMS int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
	RecordedAt string  `json:"recorded_at"`
}

// InsertTrace appends one build-task trace. RecordedAt is stamped by the caller
// (typically time.Now().UTC().Format(time.RFC3339)) so tests can inject a fixed
// timestamp without monkey-patching.
func (d *DB) InsertTrace(t Trace) error {
	const q = `INSERT INTO traces(task_id, prompt, outcome, model, effort, duration_ms, cost_usd, recorded_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := d.sql.Exec(q, t.TaskID, t.Prompt, t.Outcome, t.Model, t.Effort, t.DurationMS, t.CostUSD, t.RecordedAt); err != nil {
		return fmt.Errorf("insert trace for task %s: %w", t.TaskID, err)
	}
	return nil
}

// ListSuccessfulTraces returns traces whose outcome is "success", in insertion
// order. The caller selects all successful traces and writes the JSONL dataset.
func (d *DB) ListSuccessfulTraces() ([]Trace, error) {
	const q = `SELECT id, task_id, prompt, outcome, model, effort, duration_ms, cost_usd, recorded_at
FROM traces WHERE outcome = 'success' ORDER BY id`
	rs, err := d.sql.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list successful traces: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	var out []Trace
	for rs.Next() {
		var tr Trace
		if err := rs.Scan(&tr.ID, &tr.TaskID, &tr.Prompt, &tr.Outcome,
			&tr.Model, &tr.Effort, &tr.DurationMS, &tr.CostUSD, &tr.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rs.Err()
}

// NowUTC returns the current time in RFC3339 UTC — the timestamp format used by
// InsertTrace callers. Exported so CLI packages can stamp traces consistently.
func NowUTC() string { return time.Now().UTC().Format(time.RFC3339) }
