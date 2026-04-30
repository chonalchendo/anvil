package core

import (
	"path/filepath"
	"testing"
)

func TestAppendLink_PlanToMilestone(t *testing.T) {
	v := newScaffolded(t)
	planPath := filepath.Join(v.Root, "80-plans", "foo.q2.md")
	plan := &Artifact{
		Path: planPath,
		FrontMatter: map[string]any{
			"type": "plan", "title": "Q2", "created": "2026-04-29", "updated": "2026-04-29",
			"status": "draft", "horizon": "quarter", "target_date": "2026-06-30",
			"project": "foo",
		},
	}
	if err := plan.Save(); err != nil {
		t.Fatal(err)
	}

	if err := AppendLink(v, TypePlan, "foo.q2", TypeMilestone, "foo.m1-bar"); err != nil {
		t.Fatalf("AppendLink: %v", err)
	}

	got, err := LoadArtifact(planPath)
	if err != nil {
		t.Fatal(err)
	}
	ms, ok := got.FrontMatter["milestones"].([]any)
	if !ok {
		t.Fatalf("milestones is %T: %v", got.FrontMatter["milestones"], got.FrontMatter["milestones"])
	}
	if len(ms) != 1 || ms[0] != "[[milestone.foo.m1-bar]]" {
		t.Errorf("milestones = %v", ms)
	}
}

func TestAppendLink_Idempotent(t *testing.T) {
	v := newScaffolded(t)
	planPath := filepath.Join(v.Root, "80-plans", "foo.q2.md")
	plan := &Artifact{
		Path: planPath,
		FrontMatter: map[string]any{
			"type": "plan", "title": "Q2", "created": "2026-04-29", "updated": "2026-04-29",
			"status": "draft", "horizon": "quarter", "target_date": "2026-06-30",
			"project": "foo",
		},
	}
	plan.Save()

	for i := 0; i < 2; i++ {
		if err := AppendLink(v, TypePlan, "foo.q2", TypeMilestone, "foo.m1-bar"); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := LoadArtifact(planPath)
	ms := got.FrontMatter["milestones"].([]any)
	if len(ms) != 1 {
		t.Errorf("expected idempotent (1 entry), got %d: %v", len(ms), ms)
	}
}

func TestAppendLink_UnsupportedPair_Errors(t *testing.T) {
	v := newScaffolded(t)
	planPath := filepath.Join(v.Root, "80-plans", "foo.q2.md")
	plan := &Artifact{
		Path: planPath,
		FrontMatter: map[string]any{"type": "plan", "title": "x", "created": "2026-04-29", "status": "draft", "horizon": "quarter", "target_date": "2026-06-30", "project": "foo"},
	}
	plan.Save()

	if err := AppendLink(v, TypePlan, "foo.q2", TypeDecision, "auth.0001-x"); err == nil {
		t.Error("expected error: plan→decision not supported")
	}
}

func TestAppendLink_MissingSource_Errors(t *testing.T) {
	v := newScaffolded(t)
	if err := AppendLink(v, TypePlan, "ghost", TypeMilestone, "foo.m1-bar"); err == nil {
		t.Error("expected error for missing source")
	}
}
