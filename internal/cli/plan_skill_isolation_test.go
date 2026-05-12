package cli

import (
	"strings"
	"testing"
)

// writing-plan Phase 3 (or equivalent) must document the same-file-in-wave
// rule plus rationale and a workaround. Without this, planners over-serialise
// or trip the validator without knowing why.
func TestWritingPlanSkill_DocumentsFileIsolationRule(t *testing.T) {
	body := strings.ToLower(skillBody(t, "writing-plan"))
	for _, want := range []string{
		"same_file_in_wave",
		"file-isolation",
		"split the task",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("writing-plan/SKILL.md missing %q (file-isolation guidance)", want)
		}
	}
}
