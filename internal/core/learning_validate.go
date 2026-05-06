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
//   - exactly one type/ tag, equal to "type/learning"
//   - no status/ tag
//   - if known != nil, every non-type/ tag must be in known
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
	typeTagSeen := false
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
		if hasFacet {
			switch facet {
			case "status":
				errs = append(errs, fmt.Errorf("tag %q forbidden: status is a frontmatter field, not a tag", tag))
				continue
			case "type":
				if tag != "type/learning" {
					errs = append(errs, fmt.Errorf("type/ tag %q must equal type/learning", tag))
				}
				typeTagSeen = true
				continue
			}
		}
		if known != nil {
			if _, ok := known[tag]; !ok {
				errs = append(errs, fmt.Errorf("tag %q not in glossary; add it via `anvil tags add`", tag))
			}
		}
	}
	if !typeTagSeen {
		errs = append(errs, fmt.Errorf("learning must carry tag type/learning"))
	}

	return errs
}
