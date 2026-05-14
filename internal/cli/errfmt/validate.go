// Package errfmt provides the structured validation-error shape consumed by
// `anvil validate --json` and `anvil show --validate`.
package errfmt

// Error codes used across the CLI. Keep in sync with docs and tests.
const (
	CodeMissingRequired      = "missing_required"
	CodeMissingRequiredFacet = "missing_required_facet"
	CodeUnknownFacetValue    = "unknown_facet_value"
	CodeEnumViolation        = "enum_violation"
	CodeTypeMismatch         = "type_mismatch"
	CodeConstraintViolation  = "constraint_violation"
	CodeUnresolvedLink       = "unresolved_link"
	CodeParseError           = "parse_error"
	CodeUnknownGlossaryTag   = "unknown_glossary_tag"
)

// ValidationError is the canonical shape. Optional fields use omitempty so
// keys are absent (not null) when not applicable.
type ValidationError struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	Field    string `json:"field"`
	Got      string `json:"got"`
	Expected any    `json:"expected,omitempty"` // []string for enums; string for constraints
	Suggest  string `json:"suggest,omitempty"`  // similarity-based hint (unknown_facet_value)
	Note     string `json:"note,omitempty"`     // narrative hint (e.g. genuine novelty)
	Fix      string `json:"fix,omitempty"`
}

// NewValidationError constructs a ValidationError with the required core
// fields; optional fields are added via the With* chain.
func NewValidationError(code, path, field, got string) *ValidationError {
	return &ValidationError{Code: code, Path: path, Field: field, Got: got}
}

// WithExpected attaches the expected value (enum slice or constraint string).
func (e *ValidationError) WithExpected(expected any) *ValidationError {
	e.Expected = expected
	return e
}

// WithSuggest attaches a similarity-based hint for unknown_facet_value.
func (e *ValidationError) WithSuggest(s string) *ValidationError {
	e.Suggest = s
	return e
}

// WithNote attaches a narrative hint (e.g. genuine novelty acknowledgement).
func (e *ValidationError) WithNote(s string) *ValidationError {
	e.Note = s
	return e
}

// WithFix attaches a copy-pasteable fix string.
func (e *ValidationError) WithFix(fix string) *ValidationError {
	e.Fix = fix
	return e
}

// NewNotInVault signals that an artifact path passed to `anvil validate <file>`
// is not located under a known type-dir inside a vault.
func NewNotInVault(path string) *Structured {
	return NewStructured("not_in_vault").
		Set("path", path).
		Set("hint", "validate a vault root (`anvil validate`) or pass a file under <vault>/<type-dir>/")
}

// NewInvalidSlug signals that a user-supplied slug failed the
// `^[a-z0-9][a-z0-9-]*$` validator. Wraps the original cause so errors.Is on
// the wrapped sentinel keeps working.
func NewInvalidSlug(slug string, cause error) *Structured {
	s := NewStructured("invalid_slug").
		Set("slug", slug).
		Set("pattern", "^[a-z0-9][a-z0-9-]*$")
	if cause != nil {
		s = s.Set("cause", cause.Error()).Wrap(cause)
	}
	return s
}
