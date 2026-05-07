package index

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ArtifactRow is the in-memory shape of a row in the artifacts table.
type ArtifactRow struct {
	ID, Type, Status, Project, Path, Created, Updated string
}

// LinkRow is the in-memory shape of a row in the links table.
type LinkRow struct {
	Source, Target, Relation, Anchor string
}

var wikilinkRe = regexp.MustCompile(`^\[\[([^\]]+)\]\]$`)

// ArtifactRowFromFrontmatter projects parsed frontmatter onto an ArtifactRow.
// Returns an error if `id` is missing or non-string; everything else is
// best-effort (missing fields → empty strings).
func ArtifactRowFromFrontmatter(fm map[string]any, path string) (ArtifactRow, error) {
	id, ok := fm["id"].(string)
	if !ok || id == "" {
		return ArtifactRow{}, fmt.Errorf("frontmatter missing string `id`")
	}
	get := func(k string) string {
		s, _ := fm[k].(string)
		return s
	}
	return ArtifactRow{
		ID:      id,
		Type:    get("type"),
		Status:  get("status"),
		Project: get("project"),
		Path:    path,
		Created: get("created"),
		Updated: get("updated"),
	}, nil
}

// LinkRowsFromFrontmatter walks scalar + array frontmatter values, emits a
// LinkRow for each `[[type.id]]` wikilink, with the field name as Relation.
// Output is sorted by (Relation, Target) for deterministic comparison.
func LinkRowsFromFrontmatter(source string, fm map[string]any) []LinkRow {
	var rows []LinkRow
	for k, v := range fm {
		switch val := v.(type) {
		case string:
			if r, ok := parseWikilink(source, k, val); ok {
				rows = append(rows, r)
			}
		case []any:
			for _, e := range val {
				s, ok := e.(string)
				if !ok {
					continue
				}
				if r, ok := parseWikilink(source, k, s); ok {
					rows = append(rows, r)
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Relation != rows[j].Relation {
			return rows[i].Relation < rows[j].Relation
		}
		return rows[i].Target < rows[j].Target
	})
	return rows
}

func parseWikilink(source, relation, s string) (LinkRow, bool) {
	m := wikilinkRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return LinkRow{}, false
	}
	target := m[1]
	dot := strings.IndexByte(target, '.')
	if dot < 0 {
		return LinkRow{}, false
	}
	return LinkRow{Source: source, Target: target[dot+1:], Relation: relation, Anchor: ""}, true
}
