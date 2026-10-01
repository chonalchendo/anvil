package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/installer"
)

type skillsFixture struct {
	skillsDir, mat, retired string
}

func newSkillsFixture(t *testing.T) skillsFixture {
	t.Helper()
	root := t.TempDir()
	f := skillsFixture{
		skillsDir: filepath.Join(root, "skills"),
		mat:       filepath.Join(root, ".anvil-skills-src"),
		retired:   filepath.Join(root, "retired"),
	}
	for _, d := range []string{f.skillsDir, f.mat, f.retired} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f skillsFixture) link(t *testing.T, name, dest string) {
	t.Helper()
	if err := os.Symlink(dest, filepath.Join(f.skillsDir, name)); err != nil {
		t.Fatal(err)
	}
}

func (f skillsFixture) byID(t *testing.T) map[string]doctorFinding {
	t.Helper()
	m := map[string]doctorFinding{}
	for _, d := range checkInstalledSkills("claude", f.skillsDir, f.mat, f.retired) {
		m[d.ID] = d
	}
	return m
}

func bundledName(t *testing.T) string {
	t.Helper()
	for n := range bundleSkillNames() {
		return n
	}
	t.Fatal("empty embedded bundle")
	return ""
}

func TestCheckInstalledSkills_Entries(t *testing.T) {
	f := newSkillsFixture(t)
	if err := os.MkdirAll(filepath.Join(f.mat, "good"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.retired, "old"), 0o750); err != nil {
		t.Fatal(err)
	}
	f.link(t, "good", filepath.Join(f.mat, "good"))
	f.link(t, "old", filepath.Join(f.retired, "old"))
	f.link(t, "gone", filepath.Join(f.mat, "gone"))
	f.link(t, "mine", filepath.Join(t.TempDir(), "missing")) // foreign dangling
	bundled := bundledName(t)
	f.link(t, bundled, filepath.Join(f.retired, bundled))

	got := f.byID(t)
	if _, ok := got["good"]; ok {
		t.Errorf("healthy entry flagged: %+v", got["good"])
	}
	if _, ok := got["mine"]; ok {
		t.Errorf("foreign dangling link flagged: %+v", got["mine"])
	}
	if d, ok := got["old"]; !ok || !strings.Contains(d.Evidence, "retired") || d.Fix != "rm "+filepath.Join(f.skillsDir, "old") {
		t.Errorf("retired non-bundled entry: want rm fix, got %+v", d)
	}
	if d, ok := got["gone"]; !ok || !strings.Contains(d.Evidence, "missing") {
		t.Errorf("missing-target entry not flagged: %+v", d)
	}
	for _, id := range []string{"gone", bundled} {
		d, ok := got[id]
		if !ok {
			t.Fatalf("%s not flagged", id)
		}
		if !strings.Contains(d.Fix, "install skills --force --target claude") || !strings.Contains(d.Fix, "globally installed binary") || !strings.Contains(d.Fix, "new session") {
			t.Errorf("%s fix: %q", id, d.Fix)
		}
	}
}

func TestCheckInstalledSkills_BundleFreshness(t *testing.T) {
	f := newSkillsFixture(t)
	got := f.byID(t)
	var stale *doctorFinding
	for _, x := range got {
		if x.Kind == "stale-skills-bundle" {
			stale = &x
		}
	}
	if stale == nil || !strings.Contains(stale.Evidence, "differs from the running binary") {
		t.Fatalf("no hash file: want stale-skills-bundle, got %+v", got)
	}
	if _, err := installer.InstallSkills(skills.FS, f.mat, f.skillsDir, false, false); err != nil {
		t.Fatal(err)
	}
	if got := f.byID(t); len(got) != 0 {
		t.Errorf("fresh install should be quiet, got %+v", got)
	}
}

func TestCheckInstalledSkillsDefault_PerTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	t.Setenv("ANVIL_SKILLS_DIR", "")
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, "pi"))
	for _, tgt := range []string{"claude", "codex", "pi"} {
		sd, err := resolveAnvilSkillsTarget(tgt)
		if err != nil {
			t.Skipf("target %s unresolvable: %v", tgt, err)
		}
		mat, _ := resolveSkillsMaterialiseDir(tgt)
		if _, err := installer.InstallSkills(skills.FS, mat, sd, false, false); err != nil {
			t.Fatal(err)
		}
	}
	if got := checkInstalledSkillsDefault(); len(got) != 0 {
		t.Fatalf("fresh installs should be quiet, got %+v", got)
	}
	codexMat, _ := resolveSkillsMaterialiseDir("codex")
	if err := os.WriteFile(filepath.Join(codexMat, ".anvil-skills-hash"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := checkInstalledSkillsDefault()
	if len(got) != 1 || got[0].Kind != "stale-skills-bundle" || !strings.Contains(got[0].Fix, "--target codex") {
		t.Fatalf("want one codex stale-skills-bundle, got %+v", got)
	}
}

func TestCheckInstalledSkills_NoSkillsDir(t *testing.T) {
	if got := checkInstalledSkills("claude", filepath.Join(t.TempDir(), "nope"), "x", "y"); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// A retired-dir link whose name is bundled is cleared by the very command its
// Fix names: InstallSkills replaces anvil's symlink at that name.
func TestCheckInstalledSkills_RetiredBundledFixClears(t *testing.T) {
	f := newSkillsFixture(t)
	name := bundledName(t)
	if err := os.MkdirAll(filepath.Join(f.retired, name), 0o750); err != nil {
		t.Fatal(err)
	}
	f.link(t, name, filepath.Join(f.retired, name))
	d, ok := f.byID(t)[name]
	if !ok || !strings.Contains(d.Fix, "install skills --force") {
		t.Fatalf("want install fix, got %+v", d)
	}
	if _, err := installer.InstallSkills(skills.FS, f.mat, f.skillsDir, false, true); err != nil {
		t.Fatal(err)
	}
	if got := f.byID(t); len(got) != 0 {
		t.Errorf("fix did not clear findings: %+v", got)
	}
}
