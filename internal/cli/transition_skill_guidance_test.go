package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skillBody(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			b, err := os.ReadFile(filepath.Join(dir, "anvil", "skills", name, "SKILL.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("go.mod not found from %s", wd)
	return ""
}

// writing-issue must document the in-progress claim (with --owner) and the
// resolved transition. Without these, the agent has to guess the verb from
// CLI errors.
func TestWritingIssueSkill_DocumentsTransitions(t *testing.T) {
	body := skillBody(t, "writing-issue")
	for _, want := range []string{
		"anvil transition issue",
		"in-progress",
		"--owner",
		"resolved",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("writing-issue/SKILL.md missing %q", want)
		}
	}
}
