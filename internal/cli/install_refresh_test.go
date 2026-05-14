package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoot_AutoRefreshesStaleSkills runs `anvil install skills` against a
// temp materialiseDir, corrupts the on-disk hash marker, then runs an
// unrelated command. The root PersistentPreRunE should detect drift and
// rewrite the bundle, emitting a one-line stderr notice.
func TestRoot_AutoRefreshesStaleSkills(t *testing.T) {
	skillsRoot := t.TempDir()
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)

	vault := setupVault(t)
	_ = vault

	if _, _, err := runCmd(t, newRootCmd(), "install", "skills"); err != nil {
		t.Fatalf("install skills: %v", err)
	}

	hashPath := filepath.Join(skillsRoot, ".anvil-skills-hash")
	if err := os.WriteFile(hashPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("corrupt hash: %v", err)
	}

	_, errOut, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("where: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(errOut, "refreshed stale skills bundle") {
		t.Errorf("expected refresh notice on stderr, got:\n%s", errOut)
	}

	data, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "stale" {
		t.Error("hash file was not rewritten by auto-refresh")
	}
}

// TestRoot_AutoRefreshWarnsOncePerProcess confirms that when auto-refresh
// fails with a refusal error (non-anvil dir planted at a skill path), the
// warning fires on first invocation but is suppressed on subsequent
// invocations of refreshSkillsIfStale within the same process for the same
// offending path. The emitted warning must include the actionable hint.
func TestRoot_AutoRefreshWarnsOncePerProcess(t *testing.T) {
	skillsRoot := t.TempDir()
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)
	setupVault(t)
	resetAutoRefreshWarnedForTest()
	t.Cleanup(resetAutoRefreshWarnedForTest)

	// First, materialise a successful install so the refresh path engages.
	if _, _, err := runCmd(t, newInstallCmd(), "skills"); err != nil {
		t.Fatalf("install skills: %v", err)
	}
	// Force drift so RefreshSkillsIfStale runs.
	if err := os.WriteFile(filepath.Join(skillsRoot, ".anvil-skills-hash"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("corrupt hash: %v", err)
	}
	// Replace a materialised symlink with a foreign non-anvil dir at the
	// target so the refresh refuses.
	targetSkillsDir := filepath.Join(claudeRoot, "skills")
	entries, err := os.ReadDir(targetSkillsDir)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	var victim string
	for _, e := range entries {
		victim = e.Name()
		break
	}
	if victim == "" {
		t.Fatal("no skills materialised")
	}
	foreign := filepath.Join(targetSkillsDir, victim)
	if err := os.RemoveAll(foreign); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err1Out, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("first where: %v\nstderr: %s", err, err1Out)
	}
	_, err2Out, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("second where: %v\nstderr: %s", err, err2Out)
	}

	if !strings.Contains(err1Out, "anvil: skills auto-refresh failed") {
		t.Errorf("first invocation must warn; got stderr: %s", err1Out)
	}
	if !strings.Contains(err1Out, "anvil install skills --force") {
		t.Errorf("warning must include actionable command; got stderr: %s", err1Out)
	}
	if strings.Contains(err2Out, "anvil: skills auto-refresh failed") {
		t.Errorf("second invocation must be silent for same path; got stderr: %s", err2Out)
	}
}

// TestRoot_SkipsRefreshWhenSkillsAbsent confirms the auto-refresh is silent
// when the user never ran `anvil install skills` — no warning, no mkdir.
func TestRoot_SkipsRefreshWhenSkillsAbsent(t *testing.T) {
	skillsRoot := filepath.Join(t.TempDir(), "never-installed")
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)
	setupVault(t)

	_, errOut, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	if strings.Contains(errOut, "refreshed") {
		t.Errorf("unexpected refresh notice: %s", errOut)
	}
	if _, err := os.Stat(skillsRoot); !os.IsNotExist(err) {
		t.Errorf("auto-refresh should not have created %s: %v", skillsRoot, err)
	}
}
