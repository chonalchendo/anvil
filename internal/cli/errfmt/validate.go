// Package errfmt provides the structured validation-error shape consumed by
// `anvil validate --json` and `anvil show --validate`.
package errfmt

// ValidationError is the canonical shape. Optional fields use omitempty so
// keys are absent (not null) when not applicable.
type ValidationError struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	Field    string `json:"field"`
	Got      string `json:"got"`
	Expected any    `json:"expected,omitempty"` // []string for enums; string for constraints
	Fix      string `json:"fix,omitempty"`
}

func NewValidationError(code, path, field, got string) *ValidationError {
	return &ValidationError{Code: code, Path: path, Field: field, Got: got}
}

func (e *ValidationError) WithExpected(expected any) *ValidationError {
	e.Expected = expected
	return e
}

func (e *ValidationError) WithFix(fix string) *ValidationError {
	e.Fix = fix
	return e
}
