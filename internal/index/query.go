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
// an unresolved 'blocks' edge. The third clause keeps issues waiting behind an
// active blocker out of the pool — they belong to that blocker's owner.
// depends_on targets (prerequisites) are intentionally surfaced: an unblocked
// prerequisite is the first thing agents should pick up.
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
    WHERE l2.relation = 'blocks'
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

// SearchLearnings returns learnings whose TL;DR matches the query, ranked by
// FTS5 relevance. The query is tokenised: each whitespace-separated term is
// quoted (so punctuation can't produce an FTS syntax error) and joined with
// implicit AND, so "schema rename" requires both terms but not adjacency.
// QueryFilters Status/Project/Since/Until/Limit narrow the result; Limit ≤ 0
// returns all matches. An all-punctuation query yields no terms → no results.
func (d *DB) SearchLearnings(query string, f QueryFilters) ([]ArtifactRow, error) {
	match := ftsMatchExpr(query)
	if match == "" {
		return nil, nil
	}
	q := `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated
FROM learning_fts
JOIN artifacts a ON a.id = learning_fts.id
WHERE learning_fts MATCH ?`
	args := []any{match}
	if f.Status != "" {
		q += " AND a.status = ?"
		args = append(args, f.Status)
	}
	if f.Project != "" {
		q += " AND a.project = ?"
		args = append(args, f.Project)
	}
	if f.Since != "" {
		q += " AND a.created >= ?"
		args = append(args, f.Since)
	}
	if f.Until != "" {
		q += " AND a.created <= ?"
		args = append(args, f.Until)
	}
	q += " ORDER BY rank"
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rs, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search learnings: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
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

// SearchArtifactContent returns issues and milestones whose description+goal
// content matches the query, ranked by FTS5 relevance. Excludes excludeID so
// the calling artifact (just saved) never reports itself as its own duplicate.
// QueryFilters Type/Project narrow the result; Limit ≤ 0 returns all matches.
func (d *DB) SearchArtifactContent(query, excludeID string, f QueryFilters) ([]ArtifactRow, error) {
	match := ftsMatchExpr(query)
	if match == "" {
		return nil, nil
	}
	q := `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated
FROM artifact_fts
JOIN artifacts a ON a.id = artifact_fts.id
WHERE artifact_fts MATCH ?`
	args := []any{match}
	if excludeID != "" {
		q += " AND a.id != ?"
		args = append(args, excludeID)
	}
	if f.Status != "" {
		q += " AND a.status = ?"
		args = append(args, f.Status)
	}
	if f.Project != "" {
		q += " AND a.project = ?"
		args = append(args, f.Project)
	}
	q += " ORDER BY rank"
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rs, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search artifact content: %w", err)
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
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

// ftsMatchExpr turns a free-text query into a safe FTS5 MATCH expression:
// each whitespace term is wrapped in double quotes (embedded quotes doubled),
// joined by spaces (FTS5 implicit AND). Returns "" when no terms remain.
func ftsMatchExpr(query string) string {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return ""
	}
	for i, t := range terms {
		terms[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
	return strings.Join(terms, " ")
}

// ListByType returns every artifact of the given type, ordered by id.
func (d *DB) ListByType(typ string) ([]ArtifactRow, error) {
	const q = `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated
FROM artifacts a
WHERE a.type = ?
`
	rows, err := d.queryWithFilters(q, QueryFilters{}, []any{typ})
	if err != nil {
		return nil, fmt.Errorf("list by type %s: %w", typ, err)
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
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
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
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
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
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
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
