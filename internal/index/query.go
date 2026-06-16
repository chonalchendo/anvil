package index

import (
	"database/sql"
	"fmt"
	"sort"
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
// QueryFilters Status/Project narrow the result; Limit ≤ 0 returns all matches.
// artifact_fts mixes issues and milestones, so filtering by type is the
// caller's responsibility.
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

// RelatedRow is an artifact related to a seed, carrying the evidence for the
// relation: SharedTags are the facets it has in common with the seed, Links the
// direct link relations between the two (empty for a tag-set seed). Score ranks
// the row — higher is more related.
type RelatedRow struct {
	ArtifactRow
	Score      int
	SharedTags []string
	Links      []string
}

// relatedLinkBonus is added to a candidate's score when it is also directly
// linked to the seed (either direction). Set so one shared tag plus a direct
// link (1+2) outranks two shared tags alone (2): a direct link is the stronger
// relatedness signal.
const relatedLinkBonus = 2

// RelatedByID returns artifacts that share at least one tag with the seed,
// ranked by shared-tag count plus a bonus for a direct link to the seed. The
// seed itself is excluded. Candidates come from the facet overlap only — a
// directly-linked artifact that shares no tag is the domain of `anvil link`,
// not this verb; the link bonus only re-ranks facet candidates. Results are
// ordered Score desc, id asc (deterministic). QueryFilters narrow the candidate
// set; Limit is not applied here — the caller truncates so the total is exact.
func (d *DB) RelatedByID(id string, f QueryFilters) ([]RelatedRow, error) {
	filt, fargs := artifactFilterClauses(f)
	q := `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated,
       COUNT(DISTINCT st.tag) AS shared,
       group_concat(DISTINCT st.tag) AS shared_tags
FROM tags seed
JOIN tags st ON st.tag = seed.tag AND st.artifact <> seed.artifact
JOIN artifacts a ON a.id = st.artifact
WHERE seed.artifact = ?` + filt + `
GROUP BY a.id`
	args := append([]any{id}, fargs...)
	rows, order, err := scanRelated(d.sql, q, args)
	if err != nil {
		return nil, fmt.Errorf("related by id %s: %w", id, err)
	}

	from, err := d.LinksFrom(id)
	if err != nil {
		return nil, err
	}
	to, err := d.LinksTo(id)
	if err != nil {
		return nil, err
	}
	rels := map[string][]string{}
	for _, l := range from {
		rels[l.Target] = append(rels[l.Target], l.Relation)
	}
	for _, l := range to {
		rels[l.Source] = append(rels[l.Source], l.Relation)
	}
	for nid, rr := range rows {
		if rs, ok := rels[nid]; ok {
			rr.Score += relatedLinkBonus
			rr.Links = sortedUnique(rs)
		}
	}
	return rankRelated(rows, order), nil
}

// RelatedByTags returns artifacts carrying at least one of the given tags,
// ranked by how many of them match (SharedTags holds the matched subset). Used
// before an artifact exists — context for the facets a create is about to use.
// No link bonus applies (there is no seed artifact). Ordered Score desc, id asc.
func (d *DB) RelatedByTags(tags []string, f QueryFilters) ([]RelatedRow, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	filt, fargs := artifactFilterClauses(f)
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
	q := `
SELECT a.id, a.type, a.status, a.project, a.path, a.created, a.updated,
       COUNT(DISTINCT t.tag) AS shared,
       group_concat(DISTINCT t.tag) AS shared_tags
FROM tags t
JOIN artifacts a ON a.id = t.artifact
WHERE t.tag IN (` + placeholders + `)` + filt + `
GROUP BY a.id`
	args := make([]any, 0, len(tags)+len(fargs))
	for _, t := range tags {
		args = append(args, t)
	}
	args = append(args, fargs...)
	rows, order, err := scanRelated(d.sql, q, args)
	if err != nil {
		return nil, fmt.Errorf("related by tags: %w", err)
	}
	return rankRelated(rows, order), nil
}

// scanRelated runs a related-query and returns the rows keyed by id plus the
// insertion order, so rankRelated can apply a deterministic sort. The query
// must select the seven ArtifactRow columns then shared-count and a
// comma-joined shared-tags string.
func scanRelated(db *sql.DB, q string, args []any) (map[string]*RelatedRow, []string, error) {
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close() //nolint:errcheck // close in defer; error not actionable
	rows := map[string]*RelatedRow{}
	var order []string
	for rs.Next() {
		var r RelatedRow
		var shared int
		var sharedTags sql.NullString
		if err := rs.Scan(&r.ID, &r.Type, &r.Status, &r.Project, &r.Path, &r.Created, &r.Updated, &shared, &sharedTags); err != nil {
			return nil, nil, err
		}
		r.Score = shared
		r.SharedTags = sortedUnique(strings.Split(sharedTags.String, ","))
		rows[r.ID] = &r
		order = append(order, r.ID)
	}
	return rows, order, rs.Err()
}

// rankRelated flattens the id-keyed rows in insertion order, then sorts by
// Score desc, id asc — a stable, deterministic ranking.
func rankRelated(rows map[string]*RelatedRow, order []string) []RelatedRow {
	out := make([]RelatedRow, 0, len(order))
	for _, id := range order {
		out = append(out, *rows[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// artifactFilterClauses builds the " AND a.<col> = ?" suffix and args shared by
// the related-queries, mirroring queryWithFilters but for a GROUP BY query that
// can't reuse it. Limit is intentionally omitted — the caller truncates.
func artifactFilterClauses(f QueryFilters) (string, []any) {
	var clauses []string
	var args []any
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
	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

// sortedUnique returns the input sorted with empty strings and duplicates
// removed — used to make SharedTags / Links output deterministic.
func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
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
