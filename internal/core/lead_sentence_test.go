package core

import (
	"strings"
	"testing"
)

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

func TestValidateLeadSentence_StopsAtParagraphBoundary(t *testing.T) {
	// A short lead sentence followed by a colon-terminated line and a bullet
	// list must not be swept into the sentence scan: sentenceEndRE only
	// splits on [.?!], so an unterminated colon line would otherwise glue
	// the whole paragraph run into one over-long "sentence".
	body := "\n## Problem\nShort lead sentence here.\n\n" +
		"The following applies:\n\n" +
		"- one\n- two\n- three\n- four\n- five\n- six\n- seven\n- eight\n- nine\n- ten\n\n" +
		"## Non-goals\nng\n"
	if errs := ValidateLeadSentence(body, "Problem"); len(errs) != 0 {
		t.Errorf("ValidateLeadSentence() = %v, want none — scan must stop at the first paragraph break", errs)
	}
}

func TestValidateLeadSentence_ColonOpeningLineNotOverCounted(t *testing.T) {
	// A `## Problem` opening with a colon line followed by a bullet list is
	// a single short sentence for word-count purposes; without the
	// paragraph cut the whole block is measured as one multi-paragraph run.
	body := "\n## Problem\nThe checks below fail:\n\n" +
		"- alpha\n- beta\n- gamma\n- delta\n- epsilon\n- zeta\n- eta\n- theta\n- iota\n- kappa\n" +
		"- lambda\n- mu\n- nu\n- xi\n- omicron\n- pi\n- rho\n- sigma\n- tau\n- upsilon\n\n" +
		"## Non-goals\nng\n"
	if errs := ValidateLeadSentence(body, "Problem"); len(errs) != 0 {
		t.Errorf("ValidateLeadSentence() = %v, want none — the bullet list must not inflate the lead sentence word count", errs)
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

func TestValidateLeadSentence_BacktickOnlyMessageReadsGrammatically(t *testing.T) {
	// A backtick-only finding must not carry the dangling "is contains a
	// code span" phrasing — the verb belongs to the reason, not the shared
	// template.
	body := "\n## Objective\nShip `anvil build` for the fleet.\n\n## Non-goals\nng\n"
	errs := ValidateLeadSentence(body, "Objective")
	if len(errs) != 1 {
		t.Fatalf("ValidateLeadSentence() = %v, want exactly one finding", errs)
	}
	msg := errs[0].Error()
	if strings.Contains(msg, "is contains") {
		t.Errorf("message = %q, must not read \"is contains\"", msg)
	}
	if !strings.Contains(msg, "contains a code span") {
		t.Errorf("message = %q, want it to contain %q", msg, "contains a code span")
	}
}

func TestTruncateForError_DoesNotSplitMultiByteRune(t *testing.T) {
	// A rune that straddles the byte-limit boundary must survive intact —
	// byte-slicing would cut it, producing invalid UTF-8 in the message.
	s := strings.Repeat("a", 119) + "€" + strings.Repeat("b", 20)
	got := truncateForError(s, 120)
	if !strings.HasSuffix(got, "€…") {
		t.Errorf("truncateForError(...) = %q, want the multi-byte rune preserved before the ellipsis", got)
	}
}
