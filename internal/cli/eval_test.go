package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/index"
)

func ip(n int) *int { return &n }

func TestParseEvalFileGrading(t *testing.T) {
	data := []byte(`{"summary":{"passed":3,"failed":1,"total":4,"pass_rate":0.75}}`)
	got, err := parseEvalFile(data, "writing-product-design", "v2")
	if err != nil {
		t.Fatalf("parseEvalFile: %v", err)
	}
	want := []index.EvalRun{{
		Skill: "writing-product-design", Ref: "v2",
		Passed: ip(3), Failed: ip(1), Total: ip(4), PassRate: 0.75,
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("grading mismatch (-want +got):\n%s", diff)
	}
}

func TestParseEvalFileHistory(t *testing.T) {
	data := []byte(`{"skill_name":"pdf","current_best":"v1","iterations":[
		{"version":"v0","expectation_pass_rate":0.65},
		{"version":"v1","expectation_pass_rate":0.85}]}`)
	got, err := parseEvalFile(data, "pdf", "ignored-for-history")
	if err != nil {
		t.Fatalf("parseEvalFile: %v", err)
	}
	// Each iteration is one run keyed by its version; counts are nil.
	want := []index.EvalRun{
		{Skill: "pdf", Ref: "v0", PassRate: 0.65},
		{Skill: "pdf", Ref: "v1", PassRate: 0.85},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("history mismatch (-want +got):\n%s", diff)
	}
}

func TestParseEvalFileNeither(t *testing.T) {
	if _, err := parseEvalFile([]byte(`{"unrelated":true}`), "x", ""); err == nil {
		t.Error("want error for file with neither summary nor iterations")
	}
}

func TestParseEvalFileInvalidJSON(t *testing.T) {
	if _, err := parseEvalFile([]byte(`not json`), "x", ""); err == nil {
		t.Error("want error for invalid JSON")
	}
}
