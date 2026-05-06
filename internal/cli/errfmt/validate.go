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

func NewValidationError(code, path, field, got string) *ValidationError {
	return &ValidationError{Code: code, Path: path, Field: field, Got: got}
}

func (e *ValidationError) WithExpected(expected any) *ValidationError {
	e.Expected = expected
	return e
}

func (e *ValidationError) WithSuggest(s string) *ValidationError {
	e.Suggest = s
	return e
}

func (e *ValidationError) WithNote(s string) *ValidationError {
	e.Note = s
	return e
}

func (e *ValidationError) WithFix(fix string) *ValidationError {
	e.Fix = fix
	return e
}
