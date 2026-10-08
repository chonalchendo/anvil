package core

import (
	"strings"
	"testing"
	"time"
)

const goodMilestoneBody = "\n## Objective\nobj\n\n## Non-goals\nng\n\n## Links\nlinks\n\n## Status\nplanned\n"

func milestoneFM(acceptance []string) map[string]any {
	fm := map[string]any{
		"type":    "milestone",
		"title":   "x",
		"created": "2026-05-01",
		"status":  "planned",
		"kind":    "scoped",
	}
	anyAcc := make([]any, 0, len(acceptance))
	for _, a := range acceptance {
		anyAcc = append(anyAcc, a)
	}
	fm["acceptance"] = anyAcc
	return fm
}

func TestValidateMilestone_GoodArtifact_WithAcceptance(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM([]string{"`just install-local` exits 0"}),
		Body:        goodMilestoneBody,
	}
	if errs := ValidateMilestone(a); len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateMilestone_MissingHeading(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM([]string{"`true` exits 0"}),
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
		FrontMatter: milestoneFM([]string{"`true` exits 0"}),
		Body:        "\n## Non-goals\nng\n\n## Objective\nobj\n\n## Links\nlinks\n\n## Status\nplanned\n",
	}
	errs := ValidateMilestone(a)
	if len(errs) == 0 {
		t.Fatal("expected error for out-of-order headings")
	}
}

func TestValidateMilestone_SuccessCriteriaSectionRejected(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM([]string{"`true` exits 0"}),
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
		FrontMatter: milestoneFM([]string{"`true` exits 0"}),
		Body:        goodMilestoneBody + "\n```\n## Success criteria\nillustrative example, not a real section\n```\n",
	}
	errs := ValidateMilestone(a)
	for _, e := range errs {
		if strings.Contains(e.Error(), "Success criteria") {
			t.Errorf("errs = %v, fenced heading should not trip the refusal", errs)
		}
	}
}

func TestValidateMilestone_EmptyAcceptance_Rejected(t *testing.T) {
	a := &Artifact{
		FrontMatter: milestoneFM(nil),
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
		t.Errorf("errs = %v, want an empty-acceptance refusal", errs)
	}
}

func TestValidateMilestone_ClosedSkipsBodyChecks(t *testing.T) {
	broken := "\n## Objective\nobj\n\n## Success criteria\nold\n"
	cases := []struct {
		status  string
		wantErr bool
	}{
		{"done", false},
		{"abandoned", false},
		{"planned", true},
		{"in-progress", true},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			fm := milestoneFM(nil)
			fm["status"] = c.status
			errs := ValidateMilestone(&Artifact{FrontMatter: fm, Body: broken})
			if (len(errs) > 0) != c.wantErr {
				t.Errorf("errs = %v, wantErr %v", errs, c.wantErr)
			}
		})
	}
}

func TestMeasurementStale(t *testing.T) {
	now := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	body := func(status string) string {
		return "## Objective\no\n\n## Status\n" + status + "\n"
	}
	cases := []struct {
		name, status, body string
		want, wantOK       bool
	}{
		{"old in-progress", "in-progress", body("Measured: 2026-08-01"), true, true},
		{"fresh", "in-progress", body("Measured: 2026-10-01"), false, true},
		{"exactly threshold, mid-day now", "in-progress", body("Measured: 2026-09-19"), false, true},
		{"one day past threshold", "in-progress", body("Measured: 2026-09-18"), true, true},
		{"trailing prose", "in-progress", body("Measured: 2026-08-01 — placeholder"), true, true},
		{"date run-on digit absent", "in-progress", body("Measured: 2026-08-011"), false, false},
		{"not in-progress absent", "planned", body("Measured: 2026-08-01"), false, false},
		{"no line absent", "in-progress", body("prose"), false, false},
		{"bold form absent", "in-progress", body("**Measured:** 2026-08-01"), false, false},
		{"fenced line absent", "in-progress", body("```\nMeasured: 2026-08-01\n```"), false, false},
		{"line outside Status absent", "in-progress", "## Objective\nMeasured: 2026-08-01\n\n## Status\nprose\n", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &Artifact{FrontMatter: map[string]any{"status": c.status}, Body: c.body}
			if got, ok := MeasurementStale(a, now); got != c.want || ok != c.wantOK {
				t.Errorf("got (%v, %v), want (%v, %v)", got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestMissingMilestoneFormPartIsLineAnchored(t *testing.T) {
	cases := map[string]struct {
		body    string
		missing bool
	}{
		"whole":         {"## Objective\n\n**Design change**\n\n**Components changed:** x\n", false},
		"inline":        {"## Objective\n\nThere is no **Design change** here.\n\n**Components changed:** x\n", true},
		"fenced":        {"## Objective\n\n```\n**Design change**\n```\n\n**Components changed:** x\n", true},
		"missing parts": {"## Objective\n\nGoal.\n", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, missing := MissingMilestoneFormPart(&Artifact{Body: tc.body})
			if missing != tc.missing {
				t.Fatalf("missing = %v, want %v", missing, tc.missing)
			}
		})
	}
}
