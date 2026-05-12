package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test's working dir to find the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

// assertNoPrescriptiveCutFields fails the test if the skill text contains a
// prescriptive directive to populate any of the cut fields as frontmatter.
// "Prescriptive" means: a verb phrase that tells the author to write the
// field into frontmatter (Capture / Populate frontmatter / Add to frontmatter
// / Author X frontmatter / "All schema-required frontmatter fields present"
// enumeration). Warnings against the legacy shape ("If you wrote `X:`…") are
// allowed and expected.
func assertNoPrescriptiveCutFields(t *testing.T, text string, cutFields []string) {
	t.Helper()
	prescriptivePrefixes := []string{
		"Capture frontmatter `",
		"Capture `",
		"Populate frontmatter `",
		"Populate `",
		"Add to frontmatter `",
		"Author `",
		"frontmatter field `",
	}
	for _, f := range cutFields {
		for _, prefix := range prescriptivePrefixes {
			needle := prefix + f
			if strings.Contains(text, needle) {
				t.Errorf("skill prescribes cut frontmatter via %q", needle)
			}
		}
	}
	// The schema-required-fields enumeration in Phase 6 must not list cut
	// fields among the frontmatter keys.
	for _, marker := range []string{
		"schema-required frontmatter fields present",
		"All schema-required frontmatter fields",
	} {
		idx := strings.Index(text, marker)
		if idx < 0 {
			continue
		}
		end := idx + strings.Index(text[idx:], ")")
		if end <= idx {
			continue
		}
		enum := text[idx:end]
		for _, f := range cutFields {
			if strings.Contains(enum, "`"+f+"`") {
				t.Errorf("phase-6 enumeration lists cut field %q in %q", f, enum)
			}
		}
	}
}

// TestSkillFrontmatterAlignment_ProductDesign asserts the writing-product-design
// skill does not direct authors to populate frontmatter fields that the
// product-design schema rejects (additionalProperties: false).
func TestSkillFrontmatterAlignment_ProductDesign(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "skills", "writing-product-design", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	assertNoPrescriptiveCutFields(t, string(body), []string{
		"target_users", "problem_statement", "success_metrics", "goals",
		"constraints", "appetite", "out_of_scope", "risks", "milestones",
		"revisions",
	})
}

// TestSkillFrontmatterAlignment_SystemDesign asserts the writing-system-design
// skill does not direct authors to populate fields the schema rejects.
func TestSkillFrontmatterAlignment_SystemDesign(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "skills", "writing-system-design", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	assertNoPrescriptiveCutFields(t, string(body), []string{
		"tech_stack", "key_invariants", "authorized_decisions", "revisions", "risks",
	})
}
