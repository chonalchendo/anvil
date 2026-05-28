package cli

import (
	"strings"
	"testing"
)

// TestValidateSkill_NoDrift asserts that the current set of embedded authoring
// skills have no prescriptive frontmatter directives for schema-rejected fields.
// This replaces internal/schema/skill_alignment_test.go.
func TestValidateSkill_NoDrift(t *testing.T) {
	for skillName, typeName := range skillTypeTargets {
		skillName, typeName := skillName, typeName
		t.Run(skillName, func(t *testing.T) {
			drifts, err := checkSkillAlignment(skillName, typeName)
			if err != nil {
				t.Fatalf("checkSkillAlignment(%s): %v", skillName, err)
			}
			for _, d := range drifts {
				t.Errorf("drift: %s", d)
			}
		})
	}
}

// TestValidateSkill_DetectsDrift verifies the checker catches a deliberate
// prescriptive directive for a field the schema rejects.
func TestValidateSkill_DetectsDrift(t *testing.T) {
	// "target_users" was previously in writing-product-design but cut from the
	// product-design schema. Simulate a skill body that prescribes it.
	skillBody := "## Phase 1\nCapture frontmatter `target_users` from user input.\n"
	typeName := "product-design"

	allowed, err := schemaProperties(typeName)
	if err != nil {
		t.Fatalf("schemaProperties: %v", err)
	}
	// Verify "target_users" really is absent from the schema.
	if _, ok := allowed["target_users"]; ok {
		t.Fatal("test precondition failed: target_users is in the schema")
	}

	// Run the inline scanner against the synthetic body.
	drifts := scanForDrift("writing-product-design", typeName, skillBody, allowed)
	if len(drifts) == 0 {
		t.Error("expected drift for target_users, got none")
	}
	found := false
	for _, d := range drifts {
		if d.Field == "target_users" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected drift for target_users; got %v", drifts)
	}
}

// TestValidateSkill_AllowsSchemaFields verifies allowed fields are not flagged.
func TestValidateSkill_AllowsSchemaFields(t *testing.T) {
	// "title" and "description" are valid product-design fields.
	skillBody := "## Phase 1\nCapture frontmatter `title` and frontmatter field `description`.\n"
	typeName := "product-design"

	allowed, err := schemaProperties(typeName)
	if err != nil {
		t.Fatalf("schemaProperties: %v", err)
	}
	drifts := scanForDrift("writing-product-design", typeName, skillBody, allowed)
	if len(drifts) > 0 {
		t.Errorf("expected no drift for schema-allowed fields, got %v", drifts)
	}
}

// TestValidateSkillCmd_JSON verifies --json output is valid JSON with no findings.
func TestValidateSkillCmd_JSON(t *testing.T) {
	cmd := newRootCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"validate", "skill", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate skill --json failed: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Errorf("expected JSON array, got: %s", out.String())
	}
}

// scanForDrift is a testable extraction of checkSkillAlignment's inner loop,
// allowing unit-level injection of a synthetic skill body without rebuilding
// the embedded FS.
func scanForDrift(skillName, typeName, body string, allowed map[string]struct{}) []*skillDrift {
	var drifts []*skillDrift
	for _, prefix := range prescriptivePrefixes {
		idx := 0
		for {
			pos := strings.Index(body[idx:], prefix)
			if pos < 0 {
				break
			}
			pos += idx
			after := body[pos+len(prefix)-1:]
			m := backtickFieldRE.FindString(after)
			if m != "" {
				field := strings.Trim(m, "`")
				if _, ok := allowed[field]; !ok {
					drifts = append(drifts, &skillDrift{
						Skill: skillName,
						Type:  typeName,
						Field: field,
					})
				}
			}
			idx = pos + len(prefix)
		}
	}
	return drifts
}
