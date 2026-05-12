package facets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

// Check validates that every faceted tag in tags has a value already present
// in values, modulo the per-facet allow-set. Returns one ValidationError per
// offending tag; empty slice on accept. Tags whose facet is outside Facets
// (e.g. type/x, raw strings) are ignored.
func Check(values map[string]map[string]struct{}, tags []string, allowed map[string]bool) []*errfmt.ValidationError {
	var errs []*errfmt.ValidationError
	for _, tag := range tags {
		facet, value, hasSlash := strings.Cut(tag, "/")
		if !hasSlash || value == "" {
			continue
		}
		set, ok := values[facet]
		if !ok {
			continue
		}
		if _, present := set[value]; present {
			continue
		}
		if allowed[facet] {
			continue
		}
		errs = append(errs, buildUnknownFacetValueError(facet, tag, value, set))
	}
	return errs
}

func buildUnknownFacetValueError(facet, tag, value string, set map[string]struct{}) *errfmt.ValidationError {
	candidates := make([]string, 0, len(set))
	for v := range set {
		candidates = append(candidates, v)
	}
	sort.Strings(candidates)

	e := errfmt.NewValidationError(errfmt.CodeUnknownFacetValue, "", "tags", tag).
		WithExpected(candidates)

	if sug, ok := Suggest(value, candidates); ok {
		full := facet + "/" + sug
		e.WithSuggest(full).WithFix(fmt.Sprintf(
			"use --tags %s; run `anvil tags list --prefix %s/` to see all values, or pass --allow-new-facet=%s to introduce %q",
			full, facet, facet, value,
		))
	} else {
		e.WithNote(fmt.Sprintf(
			"no similar value in vault — likely a genuinely new %s",
			facet,
		)).WithFix(fmt.Sprintf(
			"run `anvil tags list --prefix %s/` to see existing values, or pass --allow-new-facet=%s to introduce it",
			facet, facet,
		))
	}
	return e
}
