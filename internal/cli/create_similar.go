package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// similarityThreshold gates the overlap coefficient for near-duplicate
// detection. Overlap = |A ∩ B| / min(|A|, |B|) on slug tokens longer than
// two characters. 0.5 means half the shorter title's significant tokens
// must appear in the other; high enough to ignore unrelated work, low
// enough to catch rephrasings.
const similarityThreshold = 0.5

// findNearDuplicates scans existing same-type artifacts in v and returns
// IDs whose slugs are near-duplicates of candidateID. Project-scoped types
// are filtered to the same project so cross-project titles don't collide.
// Inbox is excluded — its date prefix already namespaces and the throwaway
// nature means warnings would be noise.
func findNearDuplicates(v *core.Vault, t core.Type, project, candidateID string) []string {
	if t == core.TypeInbox || t == core.TypeDecision || t == core.TypeSession {
		return nil
	}
	dir := filepath.Join(v.Root, t.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	candidateSlug := slugFromID(t, candidateID)
	prefix := ""
	if t == core.TypeIssue || t == core.TypePlan || t == core.TypeMilestone {
		prefix = project + "."
	}
	var matches []string
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
			matches = append(matches, id)
		}
	}
	sort.Strings(matches)
	return matches
}

// slugFromID strips the type-specific prefix from id, returning the
// title-derived slug component used for similarity comparison.
func slugFromID(t core.Type, id string) string {
	switch t {
	case core.TypeIssue, core.TypePlan, core.TypeMilestone:
		if i := strings.IndexByte(id, '.'); i >= 0 {
			return id[i+1:]
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
	min := len(ta)
	if len(tb) < min {
		min = len(tb)
	}
	return float64(inter)/float64(min) >= similarityThreshold
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
