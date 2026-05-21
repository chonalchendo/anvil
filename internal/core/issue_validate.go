package core

import (
	"fmt"
	"strings"
)

// RequiredIssueSections is the ordered set of headings validate enforces on
// issue body content. H3 entries (### Direct, ### Indirect) are sub-headings
// of ## Verification and must appear after it. Exported so create can scaffold
// the skeleton without duplicating the list.
var RequiredIssueSections = []string{
	"## Problem",
	"## Acceptance criteria",
	"## Non-goals",
	"## Verification",
	"### Direct",
	"### Indirect",
	"## Links",
}

// ValidateIssue checks that the issue body contains the required headings in
// order. Same ordered-scan algorithm as ValidateLearning.
func ValidateIssue(a *Artifact) []error {
	var errs []error
	pos := 0
	body := a.Body
	for _, h := range RequiredIssueSections {
		idx := strings.Index(body[pos:], "\n"+h)
		if idx < 0 && !strings.HasPrefix(body[pos:], h) {
			errs = append(errs, fmt.Errorf("issue body missing required heading %q", h))
			continue
		}
		if idx >= 0 {
			pos = pos + idx + len(h) + 1
		} else {
			pos += len(h)
		}
	}
	return errs
}
