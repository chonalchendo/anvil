package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/schemas"
)

// schemaErrToValidationErrors walks the validation tree and collects one
// ValidationError per leaf diagnostic.
func schemaErrToValidationErrors(path string, err error) []*errfmt.ValidationError {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []*errfmt.ValidationError{
			errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", err.Error()),
		}
	}
	var out []*errfmt.ValidationError
	walkSchemaErr(path, ve, &out)
	return out
}

func walkSchemaErr(path string, ve *jsonschema.ValidationError, out *[]*errfmt.ValidationError) {
	// MinContains/Contains have causes (the failing pattern leaves), but we
	// want to emit one structured error at this level, not recurse into the
	// raw pattern failures — intercept before the generic cause-recurse.
	if _, ok := ve.ErrorKind.(*kind.MinContains); ok {
		field := strings.Join(ve.InstanceLocation, ".")
		if field == "tags" {
			pattern := tagsContainsPattern(ve)
			*out = append(*out, missingFacetErr(path, pattern))
			return
		}
	}
	if len(ve.Causes) > 0 {
		for _, c := range ve.Causes {
			walkSchemaErr(path, c, out)
		}
		return
	}
	field := strings.Join(ve.InstanceLocation, ".")
	switch k := ve.ErrorKind.(type) {
	case *kind.Required:
		for _, name := range k.Missing {
			*out = append(*out, errfmt.NewValidationError(errfmt.CodeMissingRequired, path, name, ""))
		}
	case *kind.Enum:
		e := errfmt.NewValidationError(errfmt.CodeEnumViolation, path, field, fmt.Sprint(k.Got))
		wantStrs := make([]string, 0, len(k.Want))
		for _, w := range k.Want {
			wantStrs = append(wantStrs, fmt.Sprint(w))
		}
		e.WithExpected(wantStrs)
		*out = append(*out, e)
	case *kind.Const:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeEnumViolation, path, field, fmt.Sprint(k.Got)).
			WithExpected([]string{fmt.Sprint(k.Want)}))
	case *kind.Type:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeTypeMismatch, path, field, k.Got).
			WithExpected(k.Want))
	case *kind.MinLength:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%d chars", k.Got)).
			WithExpected(fmt.Sprintf("min %d chars", k.Want)))
	case *kind.MaxLength:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%d chars", k.Got)).
			WithExpected(fmt.Sprintf("max %d chars", k.Want)))
	case *kind.Pattern:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, k.Got).
			WithExpected(fmt.Sprintf("matches pattern %s", k.Want)))
	case *kind.Format:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprint(k.Got)).
			WithExpected(fmt.Sprintf("format %s", k.Want)))
	case *kind.AdditionalProperties:
		for _, prop := range k.Properties {
			*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, prop, "unexpected").
				WithExpected("not present"))
		}
	case *kind.Contains:
		// MinContains is intercepted earlier; Contains may still arrive here
		// for the rare zero-cause path. On tags, treat it as a missing facet.
		if field == "tags" {
			*out = append(*out, missingFacetErr(path, tagsContainsPattern(ve)))
			return
		}
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	default:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	}
}

// tagsContainsPattern returns the pattern from the `contains` schema that this
// MinContains/Contains node enforces. With non-empty tags, the failing Pattern
// cause carries the pattern verbatim; with zero matching tags MinContains has
// no causes, so we resolve the pattern from the schema URL itself (which ends
// in `.../properties/tags/allOf/N` or `.../properties/tags`).
func tagsContainsPattern(ve *jsonschema.ValidationError) string {
	for _, c := range ve.Causes {
		if p, ok := c.ErrorKind.(*kind.Pattern); ok {
			return p.Want
		}
	}
	if p := patternFromSchemaURL(ve.SchemaURL); p != "" {
		return p
	}
	return "^domain/[a-z0-9-]+$"
}

// patternFromSchemaURL parses the fragment of a schema URL like
// `https://anvil.dev/schemas/<type>.schema.json#/properties/tags/allOf/N`
// and returns the contains-clause pattern at that location. Empty string on
// any parse failure — caller falls back to a default.
func patternFromSchemaURL(schemaURL string) string {
	hash := strings.Index(schemaURL, "#")
	if hash < 0 {
		return ""
	}
	resource := schemaURL[:hash]
	frag := schemaURL[hash+1:]
	const prefix = "https://anvil.dev/schemas/"
	if !strings.HasPrefix(resource, prefix) {
		return ""
	}
	name := strings.TrimPrefix(resource, prefix)
	raw, err := schemas.FS.ReadFile(name)
	if err != nil {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return ""
	}
	// resolve JSON pointer fragment manually — small alphabet, no need for a lib.
	parts := strings.Split(strings.TrimPrefix(frag, "/"), "/")
	var node any = root
	for _, p := range parts {
		if p == "" {
			continue
		}
		switch n := node.(type) {
		case map[string]any:
			node = n[p]
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= len(n) {
				return ""
			}
			node = n[idx]
		default:
			return ""
		}
	}
	// node may itself be the contains schema (allOf entry) — descend if so.
	if m, ok := node.(map[string]any); ok {
		if c, ok := m["contains"].(map[string]any); ok {
			node = c
		}
		if m2, ok := node.(map[string]any); ok {
			if p, ok := m2["pattern"].(string); ok {
				return p
			}
		}
	}
	return ""
}

// missingFacetErr builds the canonical missing_required_facet error for the
// given tags-pattern. The fix text is generic; create.go / set.go augment it
// with vault-aware hints (existing facet values, --allow-new-facet) before
// printing.
func missingFacetErr(path, pattern string) *errfmt.ValidationError {
	facet := facetNameFromPattern(pattern)
	example := fmt.Sprintf("e.g. %s/<x>", facet)
	if facet == "" {
		example = "e.g. domain/<x>"
	}
	return errfmt.NewValidationError(errfmt.CodeMissingRequiredFacet, path, "tags", "").
		WithExpected([]string{pattern}).
		WithFix(fmt.Sprintf("add a tag matching the listed pattern (%s)", example))
}

// facetNameFromPattern extracts the facet prefix (e.g. "domain", "activity")
// from a tags pattern like `^domain/[a-z0-9-]+$`. Empty string if the pattern
// doesn't follow that shape.
func facetNameFromPattern(pattern string) string {
	p := strings.TrimPrefix(pattern, "^")
	slash := strings.Index(p, "/")
	if slash <= 0 {
		return ""
	}
	return p[:slash]
}
