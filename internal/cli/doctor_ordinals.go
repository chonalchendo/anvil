package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// checkDuplicateOrdinals returns a finding per <project>.NNNN ordinal carried by
// more than one issue file. Full ids stay unambiguous, but ordinal shorthand —
// the form handoffs, PR titles and cross-references use — resolves to two
// artifacts, so the collision has to be loud rather than silently tolerated.
func checkDuplicateOrdinals(issuePaths []string) []doctorFinding {
	byOrdinal := map[string][]string{}
	for _, p := range issuePaths {
		bare := core.BareID(core.TypeIssue, strings.TrimSuffix(filepath.Base(p), ".md"))
		parts := strings.SplitN(bare, ".", 3)
		if len(parts) != 3 || !core.IsOrdinalOnly(parts[1]) {
			continue // legacy unnumbered issue — no ordinal to collide
		}
		key := parts[0] + "." + parts[1]
		byOrdinal[key] = append(byOrdinal[key], core.CanonicalID(core.TypeIssue, bare))
	}
	var findings []doctorFinding
	for key, ids := range byOrdinal {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		findings = append(findings, doctorFinding{
			Kind: "duplicate-ordinal",
			ID:   core.CanonicalID(core.TypeIssue, key),
			Evidence: fmt.Sprintf("ordinal shorthand %s resolves to %d issues: %s",
				key, len(ids), strings.Join(ids, ", ")),
			Fix: fmt.Sprintf("anvil renumber issue <id> for all but one of %s — keep the member with a landed PR (its title bakes the ordinal into git), else the one claimed in-flight",
				strings.Join(ids, ", ")),
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	return findings
}
