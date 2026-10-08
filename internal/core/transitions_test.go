package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/internal/templates"
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

func TestMilestoneDoneToPlannedIsReverse(t *testing.T) {
	tr, err := LookupTransition(TypeMilestone, "done", "planned")
	if err != nil {
		t.Fatalf("done→planned must be legal for milestones: %v", err)
	}
	if !tr.Reverse {
		t.Fatalf("expected reverse=true for done→planned reopen")
	}
}

func TestIssueTransitions_FromEscalated_OnlyOpenAndAbandonedLegal(t *testing.T) {
	tr, err := LookupTransition(TypeIssue, "in-progress", "escalated")
	if err != nil || len(tr.Requires) != 1 || tr.Requires[0] != "reason" {
		t.Fatalf("in-progress→escalated must require reason: %+v, %v", tr, err)
	}
	for _, to := range []string{"open", "abandoned"} {
		if _, err := LookupTransition(TypeIssue, "escalated", to); err != nil {
			t.Errorf("escalated→%s must be legal: %v", to, err)
		}
	}
	if _, err := LookupTransition(TypeIssue, "escalated", "resolved"); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("escalated→resolved must be illegal, got %v", err)
	}
}

func TestEveryStatusEnumValueIsReachable(t *testing.T) {
	for _, ty := range AllTypes {
		enum, err := schema.StatusEnum(string(ty))
		if err != nil {
			t.Fatal(err)
		}
		if len(enum) == 0 {
			continue
		}
		inEnum := map[string]bool{}
		for _, v := range enum {
			inEnum[v] = true
		}
		for _, tr := range transitions[ty] {
			for _, s := range []string{tr.From, tr.To} {
				if !inEnum[s] {
					t.Errorf("%s: table status %q is not in the schema enum %v", ty, s, enum)
				}
			}
		}
		seen := map[string]bool{InitialStatus(ty): true}
		queue := []string{InitialStatus(ty)}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, next := range LegalNext(ty, cur) {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		for _, v := range enum {
			if !seen[v] {
				t.Errorf("%s: enum value %q is not reachable from %q", ty, v, InitialStatus(ty))
			}
		}
	}
}

func TestTemplateStatusMatchesInitialStatus(t *testing.T) {
	for _, ty := range AllTypes {
		src, err := templates.FS.ReadFile(string(ty) + ".tmpl")
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, line := range strings.Split(string(src), "\n") {
			if v, ok := strings.CutPrefix(line, "status: "); ok {
				got = strings.TrimSpace(v)
				break
			}
		}
		if want := InitialStatus(ty); got != want {
			t.Errorf("%s: template status %q != InitialStatus %q", ty, got, want)
		}
	}
}
