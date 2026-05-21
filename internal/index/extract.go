package index

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
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

// bodyWikilinkRe matches wikilinks anywhere in body text (unanchored).
var bodyWikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// typedSlotRelations are frontmatter field names whose value is a single typed
// link to one specific artifact type. `anvil create plan` writes the `issue`
// slot as a bare id (e.g. `issue: anvil.foo`) rather than wikilink form, so
// the indexer accepts both shapes for these fields. The allowlist is scoped
// to the slot where the bug was observed; extend deliberately if other typed
// slots show the same writer/indexer mismatch.
var typedSlotRelations = map[string]bool{
	"issue": true,
}

// ArtifactRowFromFrontmatter projects parsed frontmatter onto an ArtifactRow.
// If `id` is absent or empty in frontmatter, the path stem (filename without
// extension) is used as the ID. Returns an error only if both sources yield an
// empty ID; everything else is best-effort (missing fields → empty strings).
func ArtifactRowFromFrontmatter(fm map[string]any, path string) (ArtifactRow, error) {
	id, _ := fm["id"].(string)
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	if id == "" {
		return ArtifactRow{}, fmt.Errorf("cannot derive id from frontmatter or path %q", path)
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

// LinkRowsFromBody scans body text for `[[type.id]]` wikilinks and emits one
// LinkRow per distinct target, with Relation "body" and Anchor "". Surrounding
// whitespace and a trailing `|alias` are stripped before type-prefix lookup, so
// `[[ issue.anvil.foo | Display ]]` resolves identically to `[[issue.anvil.foo]]`.
// Targets inside `![[…]]` embeds are captured the same as plain links.
// Tokens whose type prefix is not a known Anvil type are ignored. Output is
// sorted by Target for deterministic comparison.
func LinkRowsFromBody(source, body string) []LinkRow {
	matches := bodyWikilinkRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var rows []LinkRow
	for _, m := range matches {
		raw := strings.TrimSpace(m[1])
		if bar := strings.IndexByte(raw, '|'); bar >= 0 {
			raw = strings.TrimSpace(raw[:bar])
		}
		dot := strings.IndexByte(raw, '.')
		if dot < 0 {
			continue
		}
		prefix, id := raw[:dot], raw[dot+1:]
		if _, err := core.ParseType(prefix); err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, LinkRow{Source: source, Target: id, Relation: "body", Anchor: ""})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Target < rows[j].Target
	})
	return rows
}

func parseWikilink(source, relation, s string) (LinkRow, bool) {
	trimmed := strings.TrimSpace(s)
	if m := wikilinkRe.FindStringSubmatch(trimmed); m != nil {
		target := m[1]
		dot := strings.IndexByte(target, '.')
		if dot < 0 {
			return LinkRow{}, false
		}
		return LinkRow{Source: source, Target: target[dot+1:], Relation: relation, Anchor: ""}, true
	}
	// Typed-slot fallback: a bare `<project>.<slug>` id stands in for the
	// wikilink form when the field name names a single artifact type.
	if typedSlotRelations[relation] && strings.IndexByte(trimmed, '.') > 0 {
		return LinkRow{Source: source, Target: trimmed, Relation: relation, Anchor: ""}, true
	}
	return LinkRow{}, false
}
