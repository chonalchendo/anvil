package facets_test

import (
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/facets"
)

func vals(domain, activity, pattern []string) map[string]map[string]struct{} {
	out := map[string]map[string]struct{}{
		"domain": {}, "activity": {}, "pattern": {},
	}
	for _, v := range domain {
		out["domain"][v] = struct{}{}
	}
	for _, v := range activity {
		out["activity"][v] = struct{}{}
	}
	for _, v := range pattern {
		out["pattern"][v] = struct{}{}
	}
	return out
}

func TestCheck_ExistingValueAccepts(t *testing.T) {
	got := facets.Check(vals([]string{"dbt"}, nil, nil), []string{"domain/dbt"}, nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestCheck_NoFacetTagIgnored(t *testing.T) {
	got := facets.Check(vals(nil, nil, nil), []string{"randomtag", "domain/"}, nil)
	if len(got) != 0 {
		t.Errorf("non-facet tags must be ignored, got %+v", got)
	}
}

func TestCheck_ContainmentSuggestion(t *testing.T) {
	got := facets.Check(vals([]string{"dbt", "go"}, nil, nil),
		[]string{"domain/dbt-testing"}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d errors, want 1", len(got))
	}
	e := got[0]
	if e.Code != errfmt.CodeUnknownFacetValue {
		t.Errorf("code = %q", e.Code)
	}
	if e.Field != "tags" || e.Got != "domain/dbt-testing" {
		t.Errorf("field/got = %q/%q", e.Field, e.Got)
	}
	if e.Suggest != "domain/dbt" {
		t.Errorf("suggest = %q, want domain/dbt", e.Suggest)
	}
	if e.Note != "" {
		t.Errorf("note must be empty when suggest set: %q", e.Note)
	}
	if e.Fix == "" {
		t.Error("fix line missing")
	}
	if !contains(e.Fix, "anvil tags list --prefix domain/") {
		t.Errorf("fix line must point at `anvil tags list --prefix domain/`: %q", e.Fix)
	}
	if !contains(e.Fix, "--allow-new-facet=domain") {
		t.Errorf("fix line must point at --allow-new-facet=domain: %q", e.Fix)
	}
}

func TestCheck_LevenshteinSuggestion(t *testing.T) {
	got := facets.Check(vals([]string{"dbt", "go"}, nil, nil),
		[]string{"domain/dtb"}, nil)
	if len(got) != 1 || got[0].Suggest != "domain/dbt" {
		t.Fatalf("expected suggest domain/dbt, got %+v", got)
	}
}

func TestCheck_NoSimilarReturnsNote(t *testing.T) {
	got := facets.Check(vals([]string{"dbt", "go"}, nil, nil),
		[]string{"domain/quantum-physics"}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d errors, want 1", len(got))
	}
	e := got[0]
	if e.Suggest != "" {
		t.Errorf("suggest must be empty when no similar: %q", e.Suggest)
	}
	if e.Note == "" {
		t.Error("note must be set on no-similar path")
	}
	if e.Fix == "" || !contains(e.Fix, "--allow-new-facet=domain") {
		t.Errorf("fix line must point at --allow-new-facet=domain: %q", e.Fix)
	}
	if !contains(e.Fix, "anvil tags list --prefix domain/") {
		t.Errorf("fix line must point at `anvil tags list --prefix domain/`: %q", e.Fix)
	}
}

func TestCheck_AllowNewFacetSuppresses(t *testing.T) {
	allowed := map[string]bool{"domain": true}
	got := facets.Check(vals([]string{"dbt"}, nil, nil),
		[]string{"domain/quantum-physics"}, allowed)
	if len(got) != 0 {
		t.Errorf("--allow-new-facet=domain must suppress, got %+v", got)
	}
}

func TestCheck_AllowNewFacetIsPerFacet(t *testing.T) {
	allowed := map[string]bool{"domain": true}
	got := facets.Check(vals([]string{"dbt"}, nil, nil),
		[]string{"activity/research"}, allowed)
	if len(got) != 1 {
		t.Errorf("activity novelty must still trip when only domain allowed, got %+v", got)
	}
}

func TestCheck_ExpectedListIsSorted(t *testing.T) {
	got := facets.Check(vals([]string{"go", "dbt", "python"}, nil, nil),
		[]string{"domain/quantum"}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	exp, ok := got[0].Expected.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", got[0].Expected)
	}
	want := []string{"dbt", "go", "python"}
	for i := range want {
		if exp[i] != want[i] {
			t.Errorf("expected[%d] = %q, want %q", i, exp[i], want[i])
		}
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
