package core

import (
	"errors"
	"testing"
)

func TestTransitionLookupHit(t *testing.T) {
	tr, err := LookupTransition(TypeIssue, "open", "in-progress")
	if err != nil {
		t.Fatalf("expected hit: %v", err)
	}
	hasOwner := false
	for _, req := range tr.Requires {
		if req == "owner" {
			hasOwner = true
			break
		}
	}
	if !hasOwner {
		t.Fatalf("expected owner required, got %v", tr.Requires)
	}
}

func TestTransitionLookupMissReturnsErrIllegal(t *testing.T) {
	_, err := LookupTransition(TypeIssue, "open", "resolved")
	if !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition, got %v", err)
	}
}

func TestLegalNextLists(t *testing.T) {
	got := LegalNext(TypeIssue, "open")
	wantSet := map[string]bool{"in-progress": true, "abandoned": true}
	if len(got) != len(wantSet) {
		t.Fatalf("got %v want keys %v", got, wantSet)
	}
	for _, s := range got {
		if !wantSet[s] {
			t.Fatalf("unexpected %s in %v", s, got)
		}
	}
}

func TestReverseTransitionFlagged(t *testing.T) {
	tr, err := LookupTransition(TypeIssue, "resolved", "open")
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Reverse {
		t.Fatalf("expected reverse=true")
	}
}
