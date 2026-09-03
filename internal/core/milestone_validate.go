package core

import (
	"fmt"
	"strings"
)

// RequiredMilestoneSections is the ordered set of H2 headings validate
// enforces on milestone body content. Exported so create can scaffold the
// skeleton without duplicating the list.
var RequiredMilestoneSections = []string{"## Objective", "## Non-goals", "## Links", "## Status"}

// ValidateMilestone checks invariants beyond the JSON Schema:
//   - body contains the four required H2s in order (same ordered-scan
//     algorithm as ValidateIssue / ValidateLearning)
//   - body does not carry a `## Success criteria` section — `acceptance:`
//     frontmatter is the single source of truth, refined via `anvil set`
//   - a `kind: scoped` milestone does not carry an empty `acceptance` list
func ValidateMilestone(a *Artifact) []error {
	body := a.Body
	errs := scanOrderedHeadings(body, "milestone", RequiredMilestoneSections)

	// Fence-stripped so a `## Success criteria` heading quoted inside a code
	// fence (e.g. an illustrative example of the forbidden shape) doesn't
	// trip the refusal — same treatment ResolveBodyLinks gives wikilinks.
	const successHeading = "## Success criteria"
	stripped := StripFencedBlocks(body)
	if strings.Contains(stripped, "\n"+successHeading) || strings.HasPrefix(stripped, successHeading) {
		errs = append(errs, fmt.Errorf("milestone body carries %q — acceptance criteria live in the `acceptance:` frontmatter field, refined via `anvil set milestone <id> acceptance --add/--remove`, not a body section", successHeading))
	}

	kind, _ := a.FrontMatter["kind"].(string)
	acceptance, _ := a.FrontMatter["acceptance"].([]any)
	if kind == "scoped" && len(acceptance) == 0 {
		errs = append(errs, fmt.Errorf("kind: scoped milestone has empty acceptance — a scoped milestone needs a witnessable finish line; add at least one runnable-predicate acceptance criterion, or flip kind to bucket if the work is genuinely open-ended"))
	}

	return errs
}
