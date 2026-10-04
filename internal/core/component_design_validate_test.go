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
