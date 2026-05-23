package core

import (
	"strings"
	"testing"
)

// fullIssueBody is a body containing all required issue sections in order.
const fullIssueBody = "\n## Problem\np\n\n## Acceptance criteria\nac\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\nd\n\n### Indirect\ni\n\n## Links\n"

func TestValidateIssue_MissingSection(t *testing.T) {
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\n\n## Acceptance criteria\n\n## Non-goals\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for missing ## Verification and ## Links")
	}
}

func TestValidateIssue_MissingVerification(t *testing.T) {
	// Has all pre-existing sections but no ## Verification block.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\np\n\n## Acceptance criteria\nac\n\n## Non-goals\nng\n\n## Links\nlinks\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for missing ## Verification")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "## Verification") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ## Verification named in errors, got: %v", errs)
	}
}

func TestValidateIssue_MissingDirect(t *testing.T) {
	// ## Verification present but ### Direct missing (### Indirect comes first).
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\np\n\n## Acceptance criteria\nac\n\n## Non-goals\nng\n\n## Verification\n\n### Indirect\ni\n\n## Links\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for missing ### Direct")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "### Direct") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ### Direct named in errors, got: %v", errs)
	}
}

func TestValidateIssue_AllSectionsPresent(t *testing.T) {
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        fullIssueBody,
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateIssue_OutOfOrder(t *testing.T) {
	// sections present but order wrong — validator enforces ordered scan
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Links\n\n## Problem\n\n## Acceptance criteria\n\n## Non-goals\n\n## Verification\n\n### Direct\n\n### Indirect\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for out-of-order sections")
	}
}

func TestValidateIssue_NoLeadingNewline_AllSectionsPresent(t *testing.T) {
	// body with no leading newline triggers the HasPrefix branch on the first
	// heading; subsequent headings also butt up against each other, exercising
	// the pos-advance path.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "## Problem## Non-goals## Verification### Direct### Indirect## Links\n",
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("all headings present — expected no errors, got: %v", errs)
	}
}

func TestValidateIssue_NoAcceptanceCriteria_Valid(t *testing.T) {
	// `## Acceptance criteria` is demoted to an optional prose checklist: a
	// body omitting it must still validate.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\nd\n\n### Indirect\ni\n\n## Links\n",
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("AC is optional — expected no errors, got: %v", errs)
	}
}
