package facets

import (
	"sort"
	"strings"
)

// Suggest returns the best candidate match for target, or ("", false) when no
// candidate is "similar enough". A candidate matches when:
//   - either-direction substring containment (target contains candidate or
//     candidate contains target), OR
//   - Levenshtein distance <= 2.
//
// Ties are broken by ascending Levenshtein distance, then alphabetically.
func Suggest(target string, candidates []string) (string, bool) {
	if target == "" || len(candidates) == 0 {
		return "", false
	}
	type scored struct {
		val  string
		dist int
	}
	var matches []scored
	for _, c := range candidates {
		if c == "" || c == target {
			continue
		}
		d := Distance(target, c)
		contained := strings.Contains(target, c) || strings.Contains(c, target)
		if contained || d <= 2 {
			matches = append(matches, scored{val: c, dist: d})
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].dist != matches[j].dist {
			return matches[i].dist < matches[j].dist
		}
		return matches[i].val < matches[j].val
	})
	return matches[0].val, true
}
