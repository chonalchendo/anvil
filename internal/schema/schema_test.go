package schema

import (
	"maps"
	"strings"
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
		"description": "x",
		"created":     "2026-04-30", "updated": "2026-04-30",
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
		"type": "decision", "title": "Use Go", "description": "x", "created": "2026-04-29",
		"status": "accepted", "date": "2026-04-29",
		"tags": []any{"domain/dev-tools", "activity/research"},
	}
	if err := Validate("decision", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Decision_RejectsCutFields(t *testing.T) {
	for _, field := range []string{"decision-makers", "consulted", "informed", "evidence", "topic"} {
		fm := map[string]any{
			"type": "decision", "title": "X", "description": "x", "created": "2026-04-29",
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
		"type": "product-design", "title": "X", "description": "x", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
	}
	if err := Validate("product-design", fm); err != nil {
		t.Errorf("product-design valid: %v", err)
	}
}

func TestValidate_AcceptsValidSystemDesign(t *testing.T) {
	fm := map[string]any{
		"type": "system-design", "title": "X", "description": "x", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
	}
	if err := Validate("system-design", fm); err != nil {
		t.Errorf("system-design valid: %v", err)
	}
}

func TestValidate_ProductDesign_RejectsCutFields(t *testing.T) {
	for _, field := range []string{"goals", "milestones", "target_users", "revisions"} {
		fm := map[string]any{
			"type": "product-design", "title": "X", "description": "x", "created": "2026-04-29",
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
		"type": "system-design", "title": "X", "description": "x", "created": "2026-04-29",
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
		"type": "system-design", "title": "X", "description": "x", "created": "2026-04-29",
		"status": "draft", "project": "anvil",
		"authorized_decisions": []any{"[[d]]"},
	}
	if err := Validate("system-design", fm); err == nil {
		t.Error("expected rejection: authorized_decisions renamed to authorized_by")
	}
}

func TestValidate_Milestone_NewShape(t *testing.T) {
	fm := map[string]any{
		"type": "milestone", "title": "M3", "description": "x", "created": "2026-04-29",
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
			"type": "milestone", "title": "M3", "description": "x", "created": "2026-04-29",
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
		"type": "issue", "title": "Fix inbox", "description": "x", "created": "2026-04-29",
		"status": "open", "project": "anvil", "severity": "medium",
		"tags":       []any{"domain/dev-tools"},
		"milestone":  "[[milestone.anvil.cli-substrate]]",
		"acceptance": []any{"Bug reproduces no longer"},
	}
	if err := Validate("issue", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Issue_RejectsLegacyStatus(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "x", "description": "x", "created": "2026-04-29",
		"status": "external", "project": "anvil", "severity": "low",
		"tags": []any{"domain/dev-tools"},
	}
	if err := Validate("issue", fm); err == nil {
		t.Error("expected rejection: legacy status 'external' no longer valid")
	}
}

func TestValidate_Issue_RejectsBadSeverity(t *testing.T) {
	fm := map[string]any{
		"type": "issue", "title": "x", "description": "x", "created": "2026-04-29",
		"status": "open", "project": "anvil", "severity": "critical-but-not",
		"tags": []any{"domain/dev-tools"},
	}
	if err := Validate("issue", fm); err == nil {
		t.Error("expected rejection: severity must be enum")
	}
}

func TestValidate_Issue_RejectsCutFields(t *testing.T) {
	for _, field := range []string{"learnings", "discovered_in", "promoted_from"} {
		fm := map[string]any{
			"type": "issue", "title": "x", "description": "x", "created": "2026-04-29",
			"status": "open", "project": "anvil", "severity": "low",
			"tags": []any{"domain/dev-tools"},
			field:  []any{"x"},
		}
		if err := Validate("issue", fm); err == nil {
			t.Errorf("expected rejection for cut field %q", field)
		}
	}
}

func TestValidate_Plan_NewShape_AcceptsModelEffort(t *testing.T) {
	fm := map[string]any{
		"type": "plan", "id": "anvil.streaming-token-counter",
		"slug": "streaming-token-counter", "title": "x", "description": "x",
		"created": "2026-04-30", "updated": "2026-04-30",
		"status": "draft", "plan_version": 1,
		"issue": "[[issue.anvil.streaming-token-counter]]",
		"tags":  []any{"domain/dev-tools"},
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
		"type": "plan", "id": "anvil.x", "slug": "x", "title": "x", "description": "x",
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
		"type": "plan", "id": "anvil.x", "slug": "x", "title": "x", "description": "x",
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

func TestValidate_Decision_RequiresDomainAndActivityTag(t *testing.T) {
	base := map[string]any{
		"type": "decision", "title": "x", "description": "x",
		"created": "2026-05-06", "status": "accepted", "date": "2026-05-06",
	}
	t.Run("rejects domain only", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"domain/dbt"}
		if err := Validate("decision", fm); err == nil {
			t.Error("expected rejection — missing activity/")
		}
	})
	t.Run("rejects activity only", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"activity/research"}
		if err := Validate("decision", fm); err == nil {
			t.Error("expected rejection — missing domain/")
		}
	})
	t.Run("accepts both", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"domain/dbt", "activity/research"}
		if err := Validate("decision", fm); err != nil {
			t.Errorf("expected accept: %v", err)
		}
	})
}

func TestValidate_Plan_RequiresDomainTag(t *testing.T) {
	base := map[string]any{
		"type": "plan", "id": "anvil.x", "slug": "x",
		"title": "x", "description": "x",
		"created": "2026-05-06", "updated": "2026-05-06",
		"status": "draft", "plan_version": 1,
		"issue": "[[issue.anvil.x]]",
		"tasks": []any{
			map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{"a.go"}, "depends_on": []any{}, "verify": "true",
			},
		},
	}
	t.Run("rejects no domain", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{}
		if err := Validate("plan", fm); err == nil {
			t.Error("expected rejection")
		}
	})
	t.Run("accepts domain", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"domain/dev-tools"}
		if err := Validate("plan", fm); err != nil {
			t.Errorf("expected accept: %v", err)
		}
	})
}

func TestValidate_Learning_AcceptsMinimal(t *testing.T) {
	fm := map[string]any{
		"type": "learning", "title": "X", "created": "2026-04-29",
		"status": "draft", "diataxis": "explanation", "confidence": "medium",
		"tags": []any{"domain/dev-tools", "activity/research"},
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

func TestValidate_Learning_RequiresDomainAndActivityTag(t *testing.T) {
	base := map[string]any{
		"type": "learning", "title": "x", "created": "2026-05-06",
		"status": "draft", "diataxis": "explanation", "confidence": "low",
	}
	t.Run("rejects activity only", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"activity/testing"}
		if err := Validate("learning", fm); err == nil {
			t.Error("expected rejection")
		}
	})
	t.Run("accepts both", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"domain/dbt", "activity/testing"}
		if err := Validate("learning", fm); err != nil {
			t.Errorf("accept: %v", err)
		}
	})
}

func TestValidate_Thread_AcceptsMinimal(t *testing.T) {
	fm := map[string]any{
		"type": "thread", "title": "X", "created": "2026-04-29",
		"status": "open", "diataxis": "explanation",
	}
	if err := Validate("thread", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Sweep_AcceptsMinimal(t *testing.T) {
	fm := map[string]any{
		"type": "sweep", "title": "X", "description": "x", "created": "2026-04-29",
		"status": "planned", "breaking": false, "scope": "core",
	}
	if err := Validate("sweep", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}


func TestValidate_Session_AcceptsMinimal(t *testing.T) {
	fm := map[string]any{
		"type": "session", "title": "X", "created": "2026-04-29",
		"source": "claude-code", "session_id": "abc",
		"status": "distilled", "retention_until": "2026-05-29",
	}
	if err := Validate("session", fm); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidate_Inbox_AcceptsPromotedType(t *testing.T) {
	fm := map[string]any{
		"type": "inbox", "title": "x", "created": "2026-05-04",
		"status": "promoted", "promoted_to": "anvil-42", "promoted_type": "issue",
	}
	if err := Validate("inbox", fm); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestValidate_Inbox_RejectsPromotedTypeDiscard(t *testing.T) {
	fm := map[string]any{
		"type": "inbox", "title": "x", "created": "2026-05-04",
		"status": "promoted", "promoted_type": "discard",
	}
	if err := Validate("inbox", fm); err == nil {
		t.Error("expected error for promoted_type=discard, got nil")
	}
}

func TestValidate_Inbox_AcceptsAbsentPromotedType(t *testing.T) {
	fm := map[string]any{
		"type": "inbox", "title": "x", "created": "2026-05-04", "status": "raw",
	}
	if err := Validate("inbox", fm); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestValidate_Description_Required(t *testing.T) {
	cases := []struct {
		typ string
		fm  map[string]any
	}{
		{"issue", map[string]any{
			"type": "issue", "title": "x", "created": "2026-05-05",
			"status": "open", "project": "p", "severity": "low",
		}},
		{"plan", map[string]any{
			"type": "plan", "id": "anvil.x", "slug": "x", "title": "x",
			"created": "2026-05-05", "updated": "2026-05-05",
			"status": "draft", "plan_version": 1, "issue": "[[i]]",
			"tasks": []any{
				map[string]any{
					"id": "T1", "title": "x", "kind": "tdd",
					"files": []any{"a.go"}, "depends_on": []any{},
					"verify": "go test ./...",
				},
			},
		}},
		{"decision", map[string]any{
			"type": "decision", "title": "x", "created": "2026-05-05",
			"status": "accepted", "date": "2026-05-05",
		}},
		{"sweep", map[string]any{
			"type": "sweep", "title": "x", "created": "2026-05-05",
			"status": "planned", "breaking": false, "scope": "core",
		}},
		{"milestone", map[string]any{
			"type": "milestone", "title": "x", "created": "2026-05-05",
			"status": "planned", "project": "p",
		}},
		{"product-design", map[string]any{
			"type": "product-design", "title": "x", "created": "2026-05-05",
			"status": "draft", "project": "p",
		}},
		{"system-design", map[string]any{
			"type": "system-design", "title": "x", "created": "2026-05-05",
			"status": "draft", "project": "p",
		}},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			if err := Validate(c.typ, c.fm); err == nil {
				t.Fatalf("%s: expected validation error for missing description", c.typ)
			}
		})
	}
}

func TestValidate_Description_Bounds(t *testing.T) {
	base := map[string]any{
		"type": "issue", "title": "x", "created": "2026-05-05",
		"status": "open", "project": "p", "severity": "low",
		"tags": []any{"domain/dev-tools"},
	}
	cases := []struct {
		name    string
		desc    any
		wantErr bool
	}{
		{"empty", "", true},
		{"single_char", "x", false},
		{"exactly_120", strings.Repeat("a", 120), false},
		{"121", strings.Repeat("a", 121), true},
		{"newline", "line1\nline2", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm := maps.Clone(base)
			fm["description"] = c.desc
			err := Validate("issue", fm)
			if (err != nil) != c.wantErr {
				t.Fatalf("desc=%q got err=%v wantErr=%v", c.desc, err, c.wantErr)
			}
		})
	}
}

func TestValidate_Issue_RequiresDomainTag(t *testing.T) {
	base := map[string]any{
		"type": "issue", "title": "x", "description": "x",
		"created": "2026-05-06", "status": "open",
		"project": "anvil", "severity": "low",
	}

	t.Run("rejects empty tags", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{}
		if err := Validate("issue", fm); err == nil {
			t.Error("expected rejection for empty tags")
		}
	})
	t.Run("rejects no domain tag", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"pattern/idempotency"}
		if err := Validate("issue", fm); err == nil {
			t.Error("expected rejection for missing domain/")
		}
	})
	t.Run("accepts domain tag", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"domain/dbt"}
		if err := Validate("issue", fm); err != nil {
			t.Errorf("expected accept: %v", err)
		}
	})
	t.Run("rejects uppercase value", func(t *testing.T) {
		fm := maps.Clone(base)
		fm["tags"] = []any{"domain/DBT"}
		if err := Validate("issue", fm); err == nil {
			t.Error("expected rejection for uppercase domain value")
		}
	})
	t.Run("rejects missing tags entirely", func(t *testing.T) {
		fm := maps.Clone(base)
		if err := Validate("issue", fm); err == nil {
			t.Error("expected rejection for missing tags field")
		}
	})
}
