package core

import (
	"strings"
	"testing"
)

const goodMilestoneBody = "\n## Objective\nobj\n\n## Non-goals\nng\n\n## Links\nlinks\n\n## Status\nplanned\n"

func milestoneFM(kind string, acceptance []string) map[string]any {
	fm := map[string]any{
		"type":    "milestone",
		"title":   "x",
		"created": "2026-05-01",
		"status":  "planned",
		"kind":    kind,
	}
	anyAcc := make([]any, 0, len(acceptance))
	for _, a := range acceptance {
		anyAcc = append(anyAcc, a)
	}
	fm["acceptance"] = anyAcc
	return fm
}

func TestValidateMilestone_GoodArtifact_Bucket(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("bucket", nil),
		Body:        goodMilestoneBody,
	}
	if errs := ValidateMilestone(a); len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateMilestone_GoodArtifact_ScopedWithAcceptance(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("scoped", []string{"`just install-local` exits 0"}),
		Body:        goodMilestoneBody,
	}
	if errs := ValidateMilestone(a); len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateMilestone_MissingHeading(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("bucket", nil),
		Body:        "\n## Objective\nobj\n\n## Links\nlinks\n\n## Status\nplanned\n",
	}
	errs := ValidateMilestone(a)
	if len(errs) == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(errs[0].Error(), "Non-goals") {
		t.Errorf("err = %v, want mention of Non-goals", errs)
	}
}

func TestValidateMilestone_OutOfOrderHeadings(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("bucket", nil),
		Body:        "\n## Non-goals\nng\n\n## Objective\nobj\n\n## Links\nlinks\n\n## Status\nplanned\n",
	}
	errs := ValidateMilestone(a)
	if len(errs) == 0 {
		t.Fatal("expected error for out-of-order headings")
	}
}

func TestValidateMilestone_SuccessCriteriaSectionRejected(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("bucket", nil),
		Body:        goodMilestoneBody + "\n## Success criteria\nnope\n",
	}
	errs := ValidateMilestone(a)
	if len(errs) == 0 {
		t.Fatal("expected error")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "Success criteria") {
			found = true
		}
	}
	if !found {
		t.Errorf("errs = %v, want a Success criteria refusal", errs)
	}
}

func TestValidateMilestone_SuccessCriteriaInsideFence_NotRejected(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("bucket", nil),
		Body:        goodMilestoneBody + "\n```\n## Success criteria\nillustrative example, not a real section\n```\n",
	}
	errs := ValidateMilestone(a)
	for _, e := range errs {
		if strings.Contains(e.Error(), "Success criteria") {
			t.Errorf("errs = %v, fenced heading should not trip the refusal", errs)
		}
	}
}

func TestValidateMilestone_ScopedEmptyAcceptance_Rejected(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("scoped", nil),
		Body:        goodMilestoneBody,
	}
	errs := ValidateMilestone(a)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "empty acceptance") {
			found = true
		}
	}
	if !found {
		t.Errorf("errs = %v, want an empty-acceptance refusal for kind: scoped", errs)
	}
}

func TestValidateMilestone_BucketEmptyAcceptance_Allowed(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM("bucket", nil),
		Body:        goodMilestoneBody,
	}
	errs := ValidateMilestone(a)
	for _, e := range errs {
		if strings.Contains(e.Error(), "empty acceptance") {
			t.Errorf("errs = %v, kind: bucket must tolerate empty acceptance", errs)
		}
	}
}
