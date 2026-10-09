package index

import "fmt"

// SearchHit is one body-search result: the artifact row plus an FTS snippet
// whose matches sit between the control bytes \x02 and \x03.
type SearchHit struct {
	ArtifactRow
	Snippet string
}

// Search returns artifacts whose title, description, goal or body match q,
// best FTS rank first. Limit ≤ 0 returns every match. The markers survive
// HTML escaping, so the caller escapes the snippet first and then swaps them
// for markup.
func (d *DB) Search(q string, limit int) ([]SearchHit, error) {
	match := ftsMatchExpr(q)
	if match == "" {
		return nil, nil
	}
	sqlQ := `
SELECT a.id, a.type, a.status, a.project, a.title, a.path, a.created, a.updated,
       snippet(artifact_fts, -1, char(2), char(3), '…', 24)
FROM artifact_fts
JOIN artifacts a ON a.rowid = artifact_fts.rowid
WHERE artifact_fts MATCH ?
ORDER BY rank`
	args := []any{match}
	if limit > 0 {
		sqlQ += " LIMIT ?"
		args = append(args, limit)
	}
	rs, err := d.sql.Query(sqlQ, args...)
	if err != nil {
		return nil, fmt.Errorf("search bodies: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	var out []SearchHit
	for rs.Next() {
		var h SearchHit
		if err := rs.Scan(&h.ID, &h.Type, &h.Status, &h.Project, &h.Title, &h.Path, &h.Created, &h.Updated, &h.Snippet); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rs.Err()
}
