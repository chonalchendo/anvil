package schema

import (
	"testing"
)

func TestValidate_AcceptsValidInbox(t *testing.T) {
	fm := map[string]any{
		"type":    "inbox",
		"title":   "test",
		"created": "2026-04-29",
		"status":  "raw",
	}
	if err := Validate("inbox", fm); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestValidate_RejectsBadStatus(t *testing.T) {
	fm := map[string]any{
		"type": "inbox", "title": "x", "created": "2026-04-29", "status": "bogus",
	}
	if err := Validate("inbox", fm); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestValidate_RejectsMissingTitle(t *testing.T) {
	fm := map[string]any{"type": "inbox", "created": "2026-04-29", "status": "raw"}
	if err := Validate("inbox", fm); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestValidate_AcceptsValidIssue(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "x", "created": "2026-04-29", "status": "external",
	}
	if err := Validate("issue", fm); err != nil {
		t.Errorf("issue valid: %v", err)
	}
}

func TestValidate_RejectsBadIssueStatus(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "x", "created": "2026-04-29", "status": "open",
	}
	if err := Validate("issue", fm); err == nil {
		t.Error("expected error")
	}
}

func TestValidate_PlanExecutable_Valid(t *testing.T) {
	fm := map[string]any{
		"type":         "plan",
		"id":           "ANV-142",
		"slug":         "streaming-token-counter",
		"title":        "Stream-aware token counter",
		"created":      "2026-04-30",
		"updated":      "2026-04-30",
		"status":       "draft",
		"plan_version": 1,
		"milestone":    "[[milestone.telemetry.m3]]",
		"issue":        "[[issue.ANV-142]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "Define type", "kind": "tdd",
				"files": []any{"a.go", "a_test.go"},
				"depends_on": []any{},
				"verify": "go test ./...",
			},
		},
	}
	if err := Validate("plan", fm); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidate_PlanExecutable_RejectsRoadmapShape(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "ANV-1", "slug": "x", "title": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1,
		"milestone": "[[m]]", "issue": "[[i]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{"a.go"}, "depends_on": []any{},
				"verify": "go test ./...",
			},
		},
		"horizon":     "month",
		"target_date": "2026-05-15",
	}
	if err := Validate("plan", fm); err == nil {
		t.Fatal("expected validation error for legacy roadmap fields")
	}
}

func TestValidate_PlanExecutable_RequiresVerify(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "ANV-1", "slug": "x", "title": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1,
		"milestone": "[[m]]", "issue": "[[i]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{"a.go"}, "depends_on": []any{},
				// verify intentionally omitted
			},
		},
	}
	if err := Validate("plan", fm); err == nil {
		t.Fatal("expected validation error for missing verify")
	}
}


func TestValidate_AcceptsValidDecision(t *testing.T) {
	fm := map[string]any{
		"type": "decision", "title": "Use X", "created": "2026-04-29",
		"status": "accepted", "decision-makers": []any{"@alice"},
	}
	if err := Validate("decision", fm); err != nil {
		t.Errorf("decision valid: %v", err)
	}
}

func TestValidate_RejectsDecisionMissingDecisionMakers(t *testing.T) {
	fm := map[string]any{
		"type": "decision", "title": "Use X", "created": "2026-04-29",
		"status": "accepted",
	}
	if err := Validate("decision", fm); err == nil {
		t.Error("expected error: decision-makers required")
	}
}

func TestValidate_AcceptsValidProductDesign(t *testing.T) {
	fm := map[string]any{
		"type": "product-design", "title": "X", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
	}
	if err := Validate("product-design", fm); err != nil {
		t.Errorf("product-design valid: %v", err)
	}
}

func TestValidate_AcceptsValidSystemDesign(t *testing.T) {
	fm := map[string]any{
		"type": "system-design", "title": "X", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
	}
	if err := Validate("system-design", fm); err != nil {
		t.Errorf("system-design valid: %v", err)
	}
}

func TestValidate_ProductDesign_RejectsCutFields(t *testing.T) {
	for _, field := range []string{"goals", "milestones", "target_users", "revisions"} {
		fm := map[string]any{
			"type": "product-design", "title": "X", "created": "2026-04-29",
			"status": "draft", "project": "anvil",
			field: []any{"x"},
		}
		if err := Validate("product-design", fm); err == nil {
			t.Errorf("expected rejection for cut field %q", field)
		}
	}
}

func TestValidate_SystemDesign_AcceptsAuthorizedBy(t *testing.T) {
	fm := map[string]any{
		"type": "system-design", "title": "X", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
		"product_design": "[[product-design.anvil]]",
		"authorized_by":  []any{"[[decision.anvil.0001-go-rewrite]]"},
	}
	if err := Validate("system-design", fm); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidate_SystemDesign_RejectsLegacyAuthorizedDecisions(t *testing.T) {
	fm := map[string]any{
		"type": "system-design", "title": "X", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
		"authorized_decisions": []any{"[[d]]"},
	}
	if err := Validate("system-design", fm); err == nil {
		t.Error("expected rejection: authorized_decisions renamed to authorized_by")
	}
}

func TestValidate_Milestone_NewShape(t *testing.T) {
	fm := map[string]any{
		"type": "milestone", "title": "M3", "created": "2026-04-29",
		"status": "planned", "project": "anvil",
		"product_design": "[[product-design.anvil]]",
		"system_design":  "[[system-design.anvil]]",
		"authorized_by":  []any{"[[decision.anvil.0001]]"},
		"acceptance":     []any{"All issues resolved"},
	}
	if err := Validate("milestone", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Milestone_RejectsSchedulingFields(t *testing.T) {
	for _, field := range []string{"target_date", "horizon", "ordinal", "predecessors", "successors", "plans", "issues", "objectives", "risks"} {
		fm := map[string]any{
			"type": "milestone", "title": "M3", "created": "2026-04-29",
			"status": "planned", "project": "anvil",
			field: "x",
		}
		if err := Validate("milestone", fm); err == nil {
			t.Errorf("expected rejection for cut field %q", field)
		}
	}
}
