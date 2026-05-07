package index

import (
	"fmt"
	"strings"
)

// QueryFilters layers filters onto ListReady / ListOrphans. Empty fields are no-ops.
type QueryFilters struct {
	Status  string
	Project string
	Since   string
	Until   string
	Limit   int
}

// ListReady returns the random-pickup pool: issues that are open, have no
// unresolved outgoing blocks/depends_on, AND are not themselves the target of
// such an edge from a non-resolved source. The third clause keeps issues that
// someone else is already depending on out of the pool — they belong to that
// dependent's owner, not random workers. Issue-only for v0.1.
func (d *DB) ListReady(typ string, f QueryFilters) ([]ArtifactRow, error) {
	const q = `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated
FROM artifacts a
LEFT JOIN links l ON l.source = a.id AND l.relation IN ('blocks', 'depends_on')
LEFT JOIN artifacts t ON t.id = l.target
WHERE a.type = ? AND a.status = 'open'
  AND a.id NOT IN (
    SELECT l2.target FROM links l2
    JOIN artifacts src ON src.id = l2.source AND src.status NOT IN ('resolved')
    WHERE l2.relation IN ('blocks', 'depends_on')
  )
GROUP BY a.id
HAVING COUNT(CASE
    WHEN t.id IS NOT NULL AND t.status NOT IN ('resolved') THEN 1
END) = 0
`
	args := []any{typ}
	rows, err := d.queryWithFilters(q, f, args)
	if err != nil {
		return nil, fmt.Errorf("list ready: %w", err)
	}
	return rows, nil
}

// ListOrphans returns artifacts with no incoming links.
func (d *DB) ListOrphans(f QueryFilters) ([]ArtifactRow, error) {
	const q = `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated
FROM artifacts a
LEFT JOIN links l ON l.target = a.id
WHERE l.target IS NULL
`
	rows, err := d.queryWithFilters(q, f, nil)
	if err != nil {
		return nil, fmt.Errorf("list orphans: %w", err)
	}
	return rows, nil
}

func (d *DB) queryWithFilters(base string, f QueryFilters, args []any) ([]ArtifactRow, error) {
	var clauses []string
	if f.Status != "" {
		clauses = append(clauses, "a.status = ?")
		args = append(args, f.Status)
	}
	if f.Project != "" {
		clauses = append(clauses, "a.project = ?")
		args = append(args, f.Project)
	}
	if f.Since != "" {
		clauses = append(clauses, "a.created >= ?")
		args = append(args, f.Since)
	}
	if f.Until != "" {
		clauses = append(clauses, "a.created <= ?")
		args = append(args, f.Until)
	}
	q := base
	if len(clauses) > 0 {
		joiner := " AND "
		if !strings.Contains(strings.ToUpper(base), "WHERE") {
			joiner = " WHERE "
		}
		q += joiner + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY a.id"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rs, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []ArtifactRow
	for rs.Next() {
		var r ArtifactRow
		if err := rs.Scan(&r.ID, &r.Type, &r.Status, &r.Project, &r.Path, &r.Created, &r.Updated); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// LinksFrom returns outgoing edges from source.
func (d *DB) LinksFrom(source string) ([]LinkRow, error) {
	return d.linkQuery(`SELECT source, target, relation, anchor FROM links WHERE source = ? ORDER BY target, relation`, source)
}

// LinksTo returns incoming edges to target.
func (d *DB) LinksTo(target string) ([]LinkRow, error) {
	return d.linkQuery(`SELECT source, target, relation, anchor FROM links WHERE target = ? ORDER BY source, relation`, target)
}

// LinksUnresolved returns edges whose target has no row in artifacts.
func (d *DB) LinksUnresolved() ([]LinkRow, error) {
	const q = `SELECT l.source, l.target, l.relation, l.anchor
FROM links l LEFT JOIN artifacts a ON a.id = l.target
WHERE a.id IS NULL ORDER BY l.source, l.target`
	rs, err := d.sql.Query(q)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []LinkRow
	for rs.Next() {
		var r LinkRow
		if err := rs.Scan(&r.Source, &r.Target, &r.Relation, &r.Anchor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

func (d *DB) linkQuery(q, arg string) ([]LinkRow, error) {
	rs, err := d.sql.Query(q, arg)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []LinkRow
	for rs.Next() {
		var r LinkRow
		if err := rs.Scan(&r.Source, &r.Target, &r.Relation, &r.Anchor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}
