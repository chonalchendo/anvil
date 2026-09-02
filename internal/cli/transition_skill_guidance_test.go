package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillDir walks up from the working directory to the module root and
// returns the on-disk directory for the named skill.
func skillDir(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "anvil", "skills", name)
		}
	}
	t.Fatalf("go.mod not found from %s", wd)
	return ""
}

func skillBody(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(skillDir(t, name), "SKILL.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// skillBodyAndReferences concatenates a skill's SKILL.md with every file
// under its references/ directory — the shape docs get shipped in: a rule
// documented in a REQUIRED REFERENCE, not the body, still ships with the
// skill (docs/skill-authoring.md).
func skillBodyAndReferences(t *testing.T, name string) string {
	t.Helper()
	dir := skillDir(t, name)
	body := skillBody(t, name)
	refDir := filepath.Join(dir, "references")
	entries, err := os.ReadDir(refDir)
	if err != nil {
		if os.IsNotExist(err) {
			return body
		}
		t.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString(body)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(refDir, e.Name())) //nolint:gosec // path is test-controlled or application-managed; not user input
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(b)
	}
	return sb.String()
}

// writing-issue must document the in-progress claim (with --owner) and the
// resolved transition, whether in the body or in a shipped reference file.
// Without these, the agent has to guess the verb from CLI errors.
func TestWritingIssueSkill_DocumentsTransitions(t *testing.T) {
	body := skillBodyAndReferences(t, "writing-issue")
	for _, want := range []string{
		"anvil transition issue",
		"in-progress",
		"--owner",
		"resolved",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("writing-issue skill (body + references) missing %q", want)
		}
	}
}
