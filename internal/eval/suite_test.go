package eval

import (
	"testing"

	"github.com/chonalchendo/anvil/anvil/skills"
)

func TestEvalLoadSuiteFromEmbeddedBundle(t *testing.T) {
	s, err := LoadSuite(skills.FS, "extracting-skill-from-session")
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	if s.SkillName != "extracting-skill-from-session" {
		t.Errorf("SkillName = %q", s.SkillName)
	}
	if len(s.Evals) == 0 {
		t.Fatal("expected at least one eval case")
	}
	if s.Evals[0].Prompt == "" {
		t.Error("first eval has empty prompt")
	}
}

func TestEvalLoadSuiteRejectsUnknownSkill(t *testing.T) {
	if _, err := LoadSuite(skills.FS, "no-such-skill"); err == nil {
		t.Error("expected error for a skill with no evals.json")
	}
}

func TestEvalLoadSuiteRejectsTraversal(t *testing.T) {
	if _, err := LoadSuite(skills.FS, "../escape"); err == nil {
		t.Error("expected error for a path-traversal skill name")
	}
}
