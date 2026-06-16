package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// similarityThreshold gates the overlap coefficient for near-duplicate
// detection. Overlap = |A ∩ B| / min(|A|, |B|) on slug tokens longer than
// two characters. 0.5 means half the shorter title's significant tokens
// must appear in the other; high enough to ignore unrelated work, low
// enough to catch rephrasings.
const similarityThreshold = 0.5

// findNearDuplicates returns IDs of existing same-type artifacts that are likely
// near-duplicates of candidateID. Two strategies are combined: slug-token overlap
// (catches rephrasings that share significant title words) and FTS content search
// over description+goal (catches disjoint-title pairs that describe identical work).
// Project-scoped types are filtered to the same project so cross-project titles
// don't collide. Inbox is excluded — its date prefix already namespaces and the
// throwaway nature means warnings would be noise.
func findNearDuplicates(v *core.Vault, t core.Type, project, candidateID string) []string {
	if t == core.TypeInbox || t == core.TypeDecision || t == core.TypeSession {
		return nil
	}

	seen := make(map[string]struct{})

	// Strategy 1: slug-token overlap over the filesystem.
	dir := filepath.Join(v.Root, t.Dir())
	entries, err := os.ReadDir(dir)
	if err == nil {
		candidateSlug := slugFromID(t, candidateID)
		prefix := ""
		if t == core.TypeIssue || t == core.TypePlan || t == core.TypeMilestone {
			prefix = project + "."
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			id := strings.TrimSuffix(name, ".md")
			if id == candidateID {
				continue
			}
			if prefix != "" && !strings.HasPrefix(id, prefix) {
				continue
			}
			if similarSlugs(candidateSlug, slugFromID(t, id)) {
				seen[id] = struct{}{}
			}
		}
	}

	// Strategy 2: FTS content match over description+goal for issues and
	// milestones. Reads the candidate's own file to build the query; silently
	// skips if the DB or file is unavailable so create never hard-fails on
	// index absence.
	if t == core.TypeIssue || t == core.TypeMilestone {
		if hits := contentDuplicates(v, t, project, candidateID); len(hits) > 0 {
			for _, id := range hits {
				seen[id] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// contentDuplicates queries artifact_fts for existing issues/milestones whose
// description+goal content matches the candidate's own content. Returns nil on
// any index error so the caller degrades gracefully to slug-only detection.
func contentDuplicates(v *core.Vault, t core.Type, project, candidateID string) []string {
	// Read candidate's frontmatter to extract the query text.
	path := filepath.Join(v.Root, t.Dir(), candidateID+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		return nil
	}
	get := func(k string) string {
		s, _ := a.FrontMatter[k].(string)
		return strings.TrimSpace(s)
	}
	query := strings.TrimSpace(get("description") + " " + get("goal"))
	if query == "" {
		return nil
	}

	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		return nil
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	f := index.QueryFilters{Project: project}
	hits, err := db.SearchArtifactContent(query, candidateID, f)
	if err != nil {
		return nil
	}
	// Filter to same type — artifact_fts covers both issues and milestones.
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if h.Type == string(t) {
			out = append(out, h.ID)
		}
	}
	return out
}

// slugFromID strips the type-specific prefix from id, returning the
// title-derived slug component used for similarity comparison.
// For numbered issues (<project>.NNNN.<slug>) the ordinal segment is also
// stripped so similarity is measured on the slug alone.
func slugFromID(t core.Type, id string) string {
	switch t {
	case core.TypeIssue, core.TypePlan, core.TypeMilestone:
		if i := strings.IndexByte(id, '.'); i >= 0 {
			rest := id[i+1:]
			// Numbered issue: strip leading ordinal segment (all-digit token).
			if j := strings.IndexByte(rest, '.'); j >= 0 && core.IsOrdinalOnly(rest[:j]) {
				rest = rest[j+1:]
			}
			return rest
		}
	}
	return id
}

// similarSlugs reports whether two slugs share enough significant tokens to
// be flagged as near-duplicates. Significant = length > 2 characters.
func similarSlugs(a, b string) bool {
	ta := slugTokenSet(a)
	tb := slugTokenSet(b)
	if len(ta) < 2 || len(tb) < 2 {
		return false
	}
	inter := 0
	for k := range ta {
		if tb[k] {
			inter++
		}
	}
	smaller := len(ta)
	if len(tb) < smaller {
		smaller = len(tb)
	}
	return float64(inter)/float64(smaller) >= similarityThreshold
}

func slugTokenSet(slug string) map[string]bool {
	out := map[string]bool{}
	for _, tok := range strings.Split(slug, "-") {
		if len(tok) > 2 {
			out[tok] = true
		}
	}
	return out
}
