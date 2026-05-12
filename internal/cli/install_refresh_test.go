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
