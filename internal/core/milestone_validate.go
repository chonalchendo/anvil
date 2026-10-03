package core

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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

// MeasurementStaleDays is how far the Status block's `Measured:` date may
// trail today before a scoped in-progress milestone is flagged.
const MeasurementStaleDays = 14

var measuredLine = regexp.MustCompile(`(?m)^Measured: (\d{4}-\d{2}-\d{2})[ \t\r]*$`)

// MeasurementStale reports whether a scoped, in-progress milestone's `##
// Status` block was measured more than MeasurementStaleDays whole days
// before now. ok is false when the verdict does not apply — not scoped, not
// in-progress, or no parseable `Measured:` line — so callers omit the field
// rather than assert "fresh" about a milestone with no date to age.
func MeasurementStale(a *Artifact, now time.Time) (stale, ok bool) {
	status, _ := a.FrontMatter["status"].(string)
	kind, _ := a.FrontMatter["kind"].(string)
	if kind != "scoped" || status != "in-progress" {
		return false, false
	}
	m := measuredLine.FindStringSubmatch(Section(StripFencedBlocks(a.Body), "Status"))
	if m == nil {
		return false, false
	}
	measured, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return false, false
	}
	return now.UTC().Truncate(24*time.Hour).Sub(measured) > MeasurementStaleDays*24*time.Hour, true
}
