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


func TestValidate_PlanExecutable_RequiresVerify(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "ANV-1", "slug": "x", "title": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1,
		"issue": "[[i]]",
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


func TestValidate_Decision_NewShape(t *testing.T) {
	fm := map[string]any{
		"type": "decision", "title": "Use Go", "created": "2026-04-29",
		"status": "accepted", "date": "2026-04-29",
	}
	if err := Validate("decision", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Decision_RejectsCutFields(t *testing.T) {
	for _, field := range []string{"decision-makers", "consulted", "informed", "evidence", "topic"} {
		fm := map[string]any{
			"type": "decision", "title": "X", "created": "2026-04-29",
			"status": "accepted", "date": "2026-04-29",
			field: []any{"x"},
		}
		if err := Validate("decision", fm); err == nil {
			t.Errorf("expected rejection for cut field %q", field)
		}
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

func TestValidate_Issue_NewShape(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "Fix inbox", "created": "2026-04-29",
		"status": "open", "project": "anvil", "severity": "medium",
		"milestone":  "[[milestone.anvil.cli-substrate]]",
		"acceptance": []any{"Bug reproduces no longer"},
	}
	if err := Validate("issue", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Issue_RejectsLegacyStatus(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "x", "created": "2026-04-29",
		"status": "external", "project": "anvil", "severity": "low",
	}
	if err := Validate("issue", fm); err == nil {
		t.Error("expected rejection: legacy status 'external' no longer valid")
	}
}

func TestValidate_Issue_RejectsBadSeverity(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "x", "created": "2026-04-29",
		"status": "open", "project": "anvil", "severity": "critical-but-not",
	}
	if err := Validate("issue", fm); err == nil {
		t.Error("expected rejection: severity must be enum")
	}
}

func TestValidate_Issue_RejectsCutFields(t *testing.T) {
	for _, field := range []string{"learnings", "discovered_in", "promoted_from"} {
		fm := map[string]any{
			"type": "issue", "title": "x", "created": "2026-04-29",
			"status": "open", "project": "anvil", "severity": "low",
			field: []any{"x"},
		}
		if err := Validate("issue", fm); err == nil {
			t.Errorf("expected rejection for cut field %q", field)
		}
	}
}

func TestValidate_Plan_NewShape_AcceptsModelEffort(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "anvil.streaming-token-counter",
		"slug": "streaming-token-counter", "title": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1,
		"issue": "[[issue.anvil.streaming-token-counter]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{"a.go"}, "depends_on": []any{},
				"verify": "go test ./...",
				"model":  "opus-4.7",
				"effort": "high",
			},
		},
	}
	if err := Validate("plan", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Plan_RejectsMilestoneField(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "anvil.x", "slug": "x", "title": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1,
		"issue":     "[[issue.anvil.x]]",
		"milestone": "[[milestone.anvil.m1]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{"a.go"}, "depends_on": []any{},
				"verify": "go test ./...",
			},
		},
	}
	if err := Validate("plan", fm); err == nil {
		t.Error("expected rejection: plan.milestone removed")
	}
}

func TestValidate_Plan_RejectsBadModel(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "anvil.x", "slug": "x", "title": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1, "issue": "[[i]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{"a.go"}, "depends_on": []any{},
				"verify": "go test ./...",
				"model":  "gpt-5",
			},
		},
	}
	if err := Validate("plan", fm); err == nil {
		t.Error("expected rejection: model must be Anvil-supported enum")
	}
}

func TestValidate_Learning_AcceptsMinimal(t *testing.T) {
	fm := map[string]any{
		"type": "learning", "title": "X", "created": "2026-04-29",
		"status": "draft", "diataxis": "explanation", "confidence": "medium",
	}
	if err := Validate("learning", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Learning_RejectsBadEnum(t *testing.T) {
	fm := map[string]any{
		"type": "learning", "title": "X", "created": "2026-04-29",
		"status": "draft", "diataxis": "essay", "confidence": "medium",
	}
	if err := Validate("learning", fm); err == nil {
		t.Error("expected rejection: diataxis enum")
	}
}
