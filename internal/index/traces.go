package index

import "fmt"

// Trace is one recorded build-task outcome, carrying the assembled prompt and
// the outcome string from the adapter run. Used by `anvil export traces` to
// emit an eval/RL dataset (prompt + outcome JSONL).
type Trace struct {
	ID      int64  `json:"id"`
	TaskID  string `json:"task_id"`
	Prompt  string `json:"prompt"`
	Outcome string `json:"outcome"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
}

// InsertTrace appends one build-task trace.
func (d *DB) InsertTrace(t Trace) error {
	const q = `INSERT INTO traces(task_id, prompt, outcome, model, effort)
VALUES(?, ?, ?, ?, ?)`
	if _, err := d.sql.Exec(q, t.TaskID, t.Prompt, t.Outcome, t.Model, t.Effort); err != nil {
		return fmt.Errorf("insert trace for task %s: %w", t.TaskID, err)
	}
	return nil
}

// ListSuccessfulTraces returns traces whose outcome is "success", in insertion
// order. The caller selects all successful traces and writes the JSONL dataset.
func (d *DB) ListSuccessfulTraces() ([]Trace, error) {
	const q = `SELECT id, task_id, prompt, outcome, model, effort
FROM traces WHERE outcome = 'success' ORDER BY id`
	rs, err := d.sql.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list successful traces: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	var out []Trace
	for rs.Next() {
		var tr Trace
		if err := rs.Scan(&tr.ID, &tr.TaskID, &tr.Prompt, &tr.Outcome, &tr.Model, &tr.Effort); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rs.Err()
}
