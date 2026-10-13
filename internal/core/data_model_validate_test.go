package core

import (
	"strings"
	"testing"
)

// Warrant: without the ledger heading required, a data model can drop the record
// of which design each finding changes, and validate still passes.
func TestValidateDataModel_DesignDeltasRequiredBeforeOpenQuestions(t *testing.T) {
	build := func(headings []string) *Artifact {
		return &Artifact{Body: strings.Join(headings, "\n\nx\n\n") + "\n"}
	}
	full := []string{"## Domains", "## Conceptual", "## Logical", "## Physical", "## Design deltas", "## Open questions"}
	if errs := ValidateDataModel(build(full)); len(errs) != 0 {
		t.Fatalf("complete body: %v", errs)
	}
	without := []string{"## Domains", "## Conceptual", "## Logical", "## Physical", "## Open questions"}
	if errs := ValidateDataModel(build(without)); len(errs) == 0 {
		t.Error("body without ## Design deltas passed")
	}
}
