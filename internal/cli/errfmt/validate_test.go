package errfmt_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

func TestBuildValidationError_EnumViolation(t *testing.T) {
	got := errfmt.NewValidationError("enum_violation", "p.md", "status", "raw-input").
		WithExpected([]string{"raw", "promoted", "discarded"}).
		WithFix("anvil set inbox foo status=raw")

	b, _ := json.Marshal(got)
	var parsed map[string]any
	json.Unmarshal(b, &parsed)
	want := map[string]any{
		"code": "enum_violation", "path": "p.md", "field": "status", "got": "raw-input",
		"expected": []any{"raw", "promoted", "discarded"},
		"fix":      "anvil set inbox foo status=raw",
	}
	if diff := cmp.Diff(want, parsed); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildValidationError_OmitsAbsentFields(t *testing.T) {
	got := errfmt.NewValidationError("missing_required", "p.md", "description", "")
	b, _ := json.Marshal(got)
	s := string(b)
	if strings.Contains(s, "expected") || strings.Contains(s, "fix") {
		t.Errorf("absent fields should be omitted: %s", s)
	}
}
