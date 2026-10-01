package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckInstalledSkills(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	mat := filepath.Join(root, ".anvil-skills-src")
	retired := filepath.Join(root, "retired")
	for _, d := range []string{skillsDir, mat, retired} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := func(name, dest string) {
		t.Helper()
		if err := os.Symlink(dest, filepath.Join(skillsDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(mat, "good"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(retired, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	link("good", filepath.Join(mat, "good"))
	link("old", filepath.Join(retired, "old"))
	link("gone", filepath.Join(mat, "gone"))

	got := checkInstalledSkills(skillsDir, mat, retired)
	byID := map[string]doctorFinding{}
	for _, f := range got {
		byID[f.ID] = f
	}
	if _, ok := byID["good"]; ok {
		t.Errorf("healthy entry flagged: %+v", byID["good"])
	}
	if f, ok := byID["old"]; !ok || !strings.Contains(f.Evidence, "retired") {
		t.Errorf("retired-dir entry not flagged: %+v", f)
	}
	if f, ok := byID["gone"]; !ok || !strings.Contains(f.Evidence, "missing") {
		t.Errorf("missing-target entry not flagged: %+v", f)
	}
	for _, f := range got {
		if f.Kind == "stale-skills-entry" && !strings.Contains(f.Fix, "install skills --force") {
			t.Errorf("fix lacks command: %q", f.Fix)
		}
		if !strings.Contains(f.Fix, "new session") {
			t.Errorf("fix lacks new-session guidance: %q", f.Fix)
		}
	}
}

func TestCheckInstalledSkills_NoSkillsDir(t *testing.T) {
	if got := checkInstalledSkills(filepath.Join(t.TempDir(), "nope"), "x", "y"); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}
