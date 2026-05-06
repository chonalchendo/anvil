package core

import (
	"strings"
	"testing"
)

func mustFM(tags []string, extra map[string]any) map[string]any {
	fm := map[string]any{
		"type":       "learning",
		"title":      "x",
		"created":    "2026-05-01",
		"status":     "draft",
		"diataxis":   "explanation",
		"confidence": "low",
	}
	anyTags := make([]any, 0, len(tags))
	for _, t := range tags {
		anyTags = append(anyTags, t)
	}
	fm["tags"] = anyTags
	for k, v := range extra {
		fm[k] = v
	}
	return fm
}

const goodBody = "\n## TL;DR\nclaim\n\n## Evidence\nsource\n\n## Caveats\nlimit\n"

func TestValidateLearning_GoodArtifact(t *testing.T) {
	a := &Artifact{
		FrontMatter: mustFM([]string{"domain/postgres", "activity/research"}, nil),
		Body:        goodBody,
	}
	if errs := ValidateLearning(a, nil); len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateLearning_MissingBodySections(t *testing.T) {
	a := &Artifact{
		FrontMatter: mustFM([]string{"domain/postgres", "activity/research"}, nil),
		Body:        "\n## TL;DR\nclaim\n\n## Caveats\nlimit\n",
	}
	errs := ValidateLearning(a, nil)
	if len(errs) == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(errs[0].Error(), "Evidence") {
		t.Errorf("err = %v, want mention of Evidence", errs)
	}
}

func TestValidateLearning_BadTagShape(t *testing.T) {
	for _, bad := range []string{"Domain/Postgres", "domain/With Space", "domain/under_score"} {
		a := &Artifact{
			FrontMatter: mustFM([]string{"domain/postgres", "activity/research", bad}, nil),
			Body:        goodBody,
		}
		errs := ValidateLearning(a, nil)
		if len(errs) == 0 {
			t.Errorf("tag %q passed, want error", bad)
		}
	}
}

func TestValidateLearning_StatusAsTag(t *testing.T) {
	a := &Artifact{
		FrontMatter: mustFM([]string{"domain/postgres", "activity/research", "status/draft"}, nil),
		Body:        goodBody,
	}
	errs := ValidateLearning(a, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for status/ tag")
	}
}

func TestValidateLearning_NoTypeTagRequired(t *testing.T) {
	a := &Artifact{
		FrontMatter: mustFM([]string{"domain/postgres", "activity/research"}, nil),
		Body:        goodBody,
	}
	if errs := ValidateLearning(a, nil); len(errs) > 0 {
		t.Errorf("expected accept, got %v", errs)
	}
}

func TestValidateLearning_GlossaryEnforcement(t *testing.T) {
	knownTags := map[string]struct{}{"domain/postgres": {}, "activity/research": {}}

	a := &Artifact{
		FrontMatter: mustFM([]string{"domain/postgres", "activity/research", "domain/unknown"}, nil),
		Body:        goodBody,
	}
	errs := ValidateLearning(a, knownTags)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown tag")
	}

	if errs := ValidateLearning(a, nil); len(errs) != 0 {
		t.Errorf("nil glossary should skip enforcement, got %v", errs)
	}
}
