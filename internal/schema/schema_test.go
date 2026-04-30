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

func TestValidate_AcceptsValidPlan(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "title": "x", "created": "2026-04-29",
		"status": "draft", "horizon": "quarter", "target_date": "2026-06-30",
	}
	if err := Validate("plan", fm); err != nil {
		t.Errorf("plan valid: %v", err)
	}
}

func TestValidate_RejectsPlanMissingHorizon(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "title": "x", "created": "2026-04-29",
		"status": "draft", "target_date": "2026-06-30",
	}
	if err := Validate("plan", fm); err == nil {
		t.Error("expected error: horizon required")
	}
}

func TestValidate_AcceptsValidMilestone(t *testing.T) {
	fm := map[string]any{
		"type": "milestone", "title": "M1", "created": "2026-04-29",
		"status": "planned", "target_date": "2026-05-15",
	}
	if err := Validate("milestone", fm); err != nil {
		t.Errorf("milestone valid: %v", err)
	}
}

func TestValidate_RejectsMilestoneMissingTargetDate(t *testing.T) {
	fm := map[string]any{
		"type": "milestone", "title": "M1", "created": "2026-04-29",
		"status": "planned",
	}
	if err := Validate("milestone", fm); err == nil {
		t.Error("expected error: target_date required")
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
