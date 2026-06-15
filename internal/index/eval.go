package index

import (
	"fmt"
	"time"
)

// EvalRunRow is one persisted eval-run record. Duration is held as a
// time.Duration in memory and stored as float seconds in the eval_runs table.
type EvalRunRow struct {
	Skill    string        `json:"skill"`
	EvalID   int           `json:"eval_id"`
	Name     string        `json:"name"`
	Pass     bool          `json:"pass"`
	Cost     float64       `json:"cost"`
	Duration time.Duration `json:"duration"`
	Model    string        `json:"model"`
	Date     string        `json:"date"`
}

// RecordEvalRun appends one eval-run record. eval_runs is append-only history,
// so this is a plain INSERT (no upsert) — re-running an eval adds a new row.
func (d *DB) RecordEvalRun(r EvalRunRow) error {
	const q = `INSERT INTO eval_runs(skill, eval_id, name, pass, cost, duration, model, date)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := d.sql.Exec(q, r.Skill, r.EvalID, r.Name, r.Pass, r.Cost, r.Duration.Seconds(), r.Model, r.Date); err != nil {
		return fmt.Errorf("record eval run %s/%d: %w", r.Skill, r.EvalID, err)
	}
	return nil
}

// EvalHistory returns every recorded run for a skill, most recent first.
func (d *DB) EvalHistory(skill string) ([]EvalRunRow, error) {
	const q = `SELECT skill, eval_id, name, pass, cost, duration, model, date
FROM eval_runs WHERE skill = ? ORDER BY date DESC, eval_id ASC`
	rows, err := d.sql.Query(q, skill)
	if err != nil {
		return nil, fmt.Errorf("query eval history %s: %w", skill, err)
	}
	defer rows.Close() //nolint:errcheck // read-only query; close error not actionable
	var out []EvalRunRow
	for rows.Next() {
		var r EvalRunRow
		var secs float64
		if err := rows.Scan(&r.Skill, &r.EvalID, &r.Name, &r.Pass, &r.Cost, &secs, &r.Model, &r.Date); err != nil {
			return nil, fmt.Errorf("scan eval run: %w", err)
		}
		r.Duration = time.Duration(secs * float64(time.Second))
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eval history: %w", err)
	}
	return out, nil
}
