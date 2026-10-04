package core

import (
	"strings"
	"testing"
)

func TestValidateComponentDesign_MissingDoesNamedExactly(t *testing.T) {
	var sb strings.Builder
	for _, h := range RequiredComponentDesignSections {
		if h == "## Does" {
			continue
		}
		sb.WriteString(h + "\n\n")
	}
	errs := ValidateComponentDesign(&Artifact{Body: sb.String()})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), `"## Does"`) {
		t.Fatalf("want one error naming \"## Does\", got %v", errs)
	}
}

func TestValidateIssue_AnnotatedVerificationHeadingsSatisfy(t *testing.T) {
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "## Problem\n## Non-goals\n## Verification\n### Direct (unit/integration)\n### Indirect (live smoke)\n## Links\n",
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("annotated headings must satisfy: %v", errs)
	}
}
