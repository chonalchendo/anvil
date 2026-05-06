package facets_test

import (
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/facets"
)

func TestSuggest_NoCandidates(t *testing.T) {
	if _, ok := facets.Suggest("dbt", nil); ok {
		t.Error("expected no suggestion against empty candidates")
	}
}

func TestSuggest_ContainmentSubset(t *testing.T) {
	got, ok := facets.Suggest("dbt-testing", []string{"dbt", "go", "python"})
	if !ok || got != "dbt" {
		t.Errorf("Suggest = (%q, %v), want (\"dbt\", true)", got, ok)
	}
}

func TestSuggest_ContainmentSuperset(t *testing.T) {
	got, ok := facets.Suggest("dbt", []string{"dbt-testing"})
	if !ok || got != "dbt-testing" {
		t.Errorf("Suggest = (%q, %v), want (\"dbt-testing\", true)", got, ok)
	}
}

func TestSuggest_LevenshteinTypo(t *testing.T) {
	got, ok := facets.Suggest("dtb", []string{"dbt", "go", "python"})
	if !ok || got != "dbt" {
		t.Errorf("Suggest typo = (%q, %v), want (\"dbt\", true)", got, ok)
	}
}

func TestSuggest_NoMatchBeyondDistance(t *testing.T) {
	if got, ok := facets.Suggest("quantum-physics", []string{"dbt", "go", "python"}); ok {
		t.Errorf("Suggest = (%q, true), want no match", got)
	}
}

func TestSuggest_TieBreaksAlphabetically(t *testing.T) {
	got, ok := facets.Suggest("ab", []string{"ad", "ac"})
	if !ok || got != "ac" {
		t.Errorf("tie-break: Suggest = (%q, %v), want (\"ac\", true)", got, ok)
	}
}
