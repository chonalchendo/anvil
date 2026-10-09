package index

import "fmt"

// CountByType returns the artifact count per type; types with none are absent.
func (d *DB) CountByType() (map[string]int, error) {
	rs, err := d.sql.Query(`SELECT type, COUNT(*) FROM artifacts GROUP BY type`)
	if err != nil {
		return nil, fmt.Errorf("count by type: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	out := map[string]int{}
	for rs.Next() {
		var t string
		var n int
		if err := rs.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rs.Err()
}

// Projects returns the distinct non-empty project names, sorted.
func (d *DB) Projects() ([]string, error) {
	rs, err := d.sql.Query(`SELECT DISTINCT project FROM artifacts WHERE project != '' ORDER BY project`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	var out []string
	for rs.Next() {
		var p string
		if err := rs.Scan(&p); err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		out = append(out, p)
	}
	return out, rs.Err()
}
