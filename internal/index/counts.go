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

// BacklinkCounts returns, for each artifact of typ with inbound links, how many distinct artifacts link to it.
func (d *DB) BacklinkCounts(typ string) (map[string]int, error) {
	rs, err := d.sql.Query(`SELECT l.target, COUNT(DISTINCT l.source) FROM links l JOIN artifacts a ON a.id = l.target WHERE a.type = ? GROUP BY l.target`, typ)
	if err != nil {
		return nil, fmt.Errorf("backlink counts: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	out := map[string]int{}
	for rs.Next() {
		var id string
		var n int
		if err := rs.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rs.Err()
}

// TagsByType returns each tagged artifact of typ with its tags, sorted.
func (d *DB) TagsByType(typ string) (map[string][]string, error) {
	rs, err := d.sql.Query(`SELECT t.artifact, t.tag FROM tags t JOIN artifacts a ON a.id = t.artifact WHERE a.type = ? ORDER BY t.artifact, t.tag`, typ)
	if err != nil {
		return nil, fmt.Errorf("tags by type: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	out := map[string][]string{}
	for rs.Next() {
		var id, tag string
		if err := rs.Scan(&id, &tag); err != nil {
			return nil, err
		}
		out[id] = append(out[id], tag)
	}
	return out, rs.Err()
}

// RecentlyUpdated returns the n most recently updated non-session artifacts, newest first; ties break by id.
func (d *DB) RecentlyUpdated(n int) ([]ArtifactRow, error) {
	rs, err := d.sql.Query(`SELECT id, type, status, project, title, path, created, updated FROM artifacts WHERE type != 'session' ORDER BY updated DESC, id LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("recently updated: %w", err)
	}
	return scanArtifactRows(rs)
}

// CountByTypeStatus returns one project's artifact counts keyed by type, then status.
func (d *DB) CountByTypeStatus(project string) (map[string]map[string]int, error) {
	rs, err := d.sql.Query(`SELECT type, status, COUNT(*) FROM artifacts WHERE project = ? GROUP BY type, status`, project)
	if err != nil {
		return nil, fmt.Errorf("count by type and status: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	out := map[string]map[string]int{}
	for rs.Next() {
		var t, st string
		var n int
		if err := rs.Scan(&t, &st, &n); err != nil {
			return nil, err
		}
		if out[t] == nil {
			out[t] = map[string]int{}
		}
		out[t][st] = n
	}
	return out, rs.Err()
}

// MilestoneIssueCounts returns, for each milestone that has issues in project,
// the issues linked through the `milestone` slot and how many are resolved.
func (d *DB) MilestoneIssueCounts(project string) (map[string]MilestoneStatus, error) {
	const q = `
SELECT l.target, COUNT(*), COUNT(CASE WHEN a.status = 'resolved' THEN 1 END)
FROM links l
JOIN artifacts a ON a.id = l.source AND a.type = 'issue'
WHERE l.relation = 'milestone' AND a.project = ?
GROUP BY l.target`
	rs, err := d.sql.Query(q, project)
	if err != nil {
		return nil, fmt.Errorf("milestone issue counts: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	out := map[string]MilestoneStatus{}
	for rs.Next() {
		m := MilestoneStatus{}
		if err := rs.Scan(&m.Milestone, &m.Total, &m.Resolved); err != nil {
			return nil, err
		}
		m.Done = m.Total > 0 && m.Resolved == m.Total
		out[m.Milestone] = m
	}
	return out, rs.Err()
}
