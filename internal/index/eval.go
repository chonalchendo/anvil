package index

import (
	"database/sql"
	"fmt"
)

// EvalRun is one recorded eval result for a skill. Counts are nil when the
// source provides only a pass-rate (skill-creator history.json iterations
// carry expectation_pass_rate but no passed/failed/total).
type EvalRun struct {
	Skill    string  `json:"skill"`
	Source   string  `json:"source"`
	Ref      string  `json:"ref"`
	Passed   *int    `json:"passed"`
	Failed   *int    `json:"failed"`
	Total    *int    `json:"total"`
	PassRate float64 `json:"pass_rate"`
	Date     string  `json:"date"`
}

// InsertEvalRun appends one eval result. Rows are append-only: a skill
// accumulates a history of runs over time.
func (d *DB) InsertEvalRun(r EvalRun) error {
	const q = `INSERT INTO eval_runs(skill, source, ref, passed, failed, total, pass_rate, date)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := d.sql.Exec(q, r.Skill, r.Source, r.Ref,
		nullableInt(r.Passed), nullableInt(r.Failed), nullableInt(r.Total),
		r.PassRate, r.Date); err != nil {
		return fmt.Errorf("insert eval run for %s: %w", r.Skill, err)
	}
	return nil
}

// EvalHistory returns a skill's recorded runs in insertion (chronological) order.
func (d *DB) EvalHistory(skill string) ([]EvalRun, error) {
	const q = `SELECT skill, source, ref, passed, failed, total, pass_rate, date
FROM eval_runs WHERE skill = ? ORDER BY rowid`
	rs, err := d.sql.Query(q, skill)
	if err != nil {
		return nil, fmt.Errorf("eval history for %s: %w", skill, err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	var out []EvalRun
	for rs.Next() {
		var r EvalRun
		var passed, failed, total sql.NullInt64
		if err := rs.Scan(&r.Skill, &r.Source, &r.Ref, &passed, &failed, &total, &r.PassRate, &r.Date); err != nil {
			return nil, err
		}
		r.Passed, r.Failed, r.Total = intPtr(passed), intPtr(failed), intPtr(total)
		out = append(out, r)
	}
	return out, rs.Err()
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}
