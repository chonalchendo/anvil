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

func TestBuildValidationError_WithSuggestAndNote(t *testing.T) {
	got := errfmt.NewValidationError(errfmt.CodeUnknownFacetValue, "p.md", "tags", "domain/dbt-testing").
		WithExpected([]string{"dbt", "go"}).
		WithSuggest("domain/dbt").
		WithFix("use --tags domain/dbt, or pass --allow-new-facet=domain")

	b, _ := json.Marshal(got)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"code": "unknown_facet_value", "path": "p.md", "field": "tags",
		"got":      "domain/dbt-testing",
		"expected": []any{"dbt", "go"},
		"suggest":  "domain/dbt",
		"fix":      "use --tags domain/dbt, or pass --allow-new-facet=domain",
	}
	if diff := cmp.Diff(want, parsed); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildValidationError_WithNote_OmitsSuggest(t *testing.T) {
	got := errfmt.NewValidationError(errfmt.CodeUnknownFacetValue, "p.md", "tags", "domain/quantum").
		WithExpected([]string{"dbt"}).
		WithNote("no similar value in vault — likely a genuinely new domain").
		WithFix("pass --allow-new-facet=domain")

	b, _ := json.Marshal(got)
	s := string(b)
	if strings.Contains(s, "suggest") {
		t.Errorf("suggest must be omitted when not set: %s", s)
	}
	if !strings.Contains(s, `"note":"no similar value in vault`) {
		t.Errorf("note missing: %s", s)
	}
}
