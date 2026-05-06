package core

import (
	"fmt"
	"regexp"
	"strings"
)

var tagShape = regexp.MustCompile(`^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)?$`)

var requiredLearningSections = []string{"## TL;DR", "## Evidence", "## Caveats"}

// ValidateLearning checks invariants beyond the JSON Schema:
//   - body contains the three required H2s in order
//   - tag values match tagShape (lowercase ASCII + hyphen)
//   - no status/ tag (status is a frontmatter field)
//   - if known != nil, every tag must be in known
func ValidateLearning(a *Artifact, known map[string]struct{}) []error {
	var errs []error

	pos := 0
	body := a.Body
	for _, h := range requiredLearningSections {
		idx := strings.Index(body[pos:], "\n"+h)
		if idx < 0 && !strings.HasPrefix(body[pos:], h) {
			errs = append(errs, fmt.Errorf("learning body missing required heading %q", h))
			continue
		}
		if idx >= 0 {
			pos = pos + idx + len(h) + 1
		} else {
			pos = len(h)
		}
	}

	tagsRaw, _ := a.FrontMatter["tags"].([]any)
	for _, raw := range tagsRaw {
		tag, ok := raw.(string)
		if !ok {
			errs = append(errs, fmt.Errorf("non-string tag value: %v", raw))
			continue
		}
		if !tagShape.MatchString(tag) {
			errs = append(errs, fmt.Errorf("tag %q must be lowercase ASCII + hyphen (a-z0-9-), optional one /", tag))
			continue
		}
		facet, _, hasFacet := strings.Cut(tag, "/")
		if tag == "status" || (hasFacet && facet == "status") {
			errs = append(errs, fmt.Errorf("tag %q forbidden: status is a frontmatter field, not a tag", tag))
			continue
		}
		if known != nil {
			if _, ok := known[tag]; !ok {
				errs = append(errs, fmt.Errorf(
					"tag %q not in glossary; add it via `anvil tags add %s --desc \"...\"`",
					tag, tag,
				))
			}
		}
	}

	return errs
}
