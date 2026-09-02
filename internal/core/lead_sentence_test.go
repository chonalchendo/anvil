package core

import "testing"

func TestValidateLeadSentence_OverLengthFlagged(t *testing.T) {
	body := "\n## Problem\n" +
		"This opening sentence keeps going well past the twenty five word limit that the writing issue skill now prescribes for a lead sentence so the validator has to notice it.\n\n" +
		"## Non-goals\nng\n"
	errs := ValidateLeadSentence(body, "Problem")
	if len(errs) != 1 {
		t.Fatalf("ValidateLeadSentence() = %v, want exactly one finding", errs)
	}
}

func TestValidateLeadSentence_BacktickFlagged(t *testing.T) {
	body := "\n## Objective\nShip `anvil build` for the fleet.\n\n## Non-goals\nng\n"
	errs := ValidateLeadSentence(body, "Objective")
	if len(errs) != 1 {
		t.Fatalf("ValidateLeadSentence() = %v, want exactly one finding", errs)
	}
}

func TestValidateLeadSentence_CleanShortSentence(t *testing.T) {
	body := "\n## Problem\nNothing checks the opening sentence of an issue.\n\n## Non-goals\nng\n"
	if errs := ValidateLeadSentence(body, "Problem"); len(errs) != 0 {
		t.Errorf("ValidateLeadSentence() = %v, want none", errs)
	}
}

func TestValidateLeadSentence_MissingHeadingReturnsNil(t *testing.T) {
	body := "\n## Non-goals\nng\n"
	if errs := ValidateLeadSentence(body, "Problem"); len(errs) != 0 {
		t.Errorf("ValidateLeadSentence() = %v, want none for absent heading", errs)
	}
}

func TestValidateLeadSentence_ScopedBeforeFirstFence(t *testing.T) {
	// A short lead sentence followed by a long fenced code block must not
	// leak fence content into the sentence scan (the fence's own text would
	// otherwise blow the word count).
	body := "\n## Problem\nShort lead sentence here.\n\n```bash\n" +
		"this is a very long line inside a fenced code block that would " +
		"otherwise push the word count well past the twenty five word cap " +
		"if the scan did not stop at the fence boundary\n```\n\n## Non-goals\nng\n"
	if errs := ValidateLeadSentence(body, "Problem"); len(errs) != 0 {
		t.Errorf("ValidateLeadSentence() = %v, want none — fence must not extend the sentence scan", errs)
	}
}
