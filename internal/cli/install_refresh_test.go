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
	if err := os.WriteFile(hashPath, []byte("stale"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatalf("corrupt hash: %v", err)
	}

	_, errOut, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("where: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(errOut, "refreshed stale skills bundle") {
		t.Errorf("expected refresh notice on stderr, got:\n%s", errOut)
	}

	data, err := os.ReadFile(hashPath) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "stale" {
		t.Error("hash file was not rewritten by auto-refresh")
	}
}

// TestAutoRefreshNonSymlinkQuiet confirms the implicit auto-refresh path
// is silent when a target/<name> entry is a user-owned regular directory
// rather than a symlink. The detailed "refusing to overwrite" warning is
// reserved for the explicit `anvil install skills` command — the implicit
// refresh must not flood stderr on every invocation.
func TestAutoRefreshNonSymlinkQuiet(t *testing.T) {
	skillsRoot := t.TempDir()
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)
	setupVault(t)

	// Materialise a successful install so the refresh path engages.
	if _, _, err := runCmd(t, newInstallCmd(), "skills"); err != nil {
		t.Fatalf("install skills: %v", err)
	}
	// Force drift so RefreshSkillsIfStale runs.
	if err := os.WriteFile(filepath.Join(skillsRoot, ".anvil-skills-hash"), []byte("stale"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatalf("corrupt hash: %v", err)
	}
	// Replace a materialised symlink with a regular non-anvil dir at the
	// target so the refresh would refuse if it surfaced the error.
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
	if err := os.MkdirAll(foreign, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}

	// Two back-to-back invocations of an unrelated command — neither may
	// emit the refusal warning on stderr.
	for i := 0; i < 2; i++ {
		_, errOut, err := runCmd(t, newRootCmd(), "where")
		if err != nil {
			t.Fatalf("where invocation %d: %v\nstderr: %s", i+1, err, errOut)
		}
		if strings.Contains(errOut, "refusing to overwrite") {
			t.Errorf("invocation %d leaked refusal warning to stderr: %s", i+1, errOut)
		}
		if strings.Contains(errOut, "skills auto-refresh failed") {
			t.Errorf("invocation %d emitted auto-refresh failure for benign non-symlink: %s", i+1, errOut)
		}
	}

	// The explicit install path must still surface the refusal.
	_, errOut, runErr := runCmd(t, newInstallCmd(), "skills")
	if runErr == nil {
		t.Fatal("explicit `install skills` must refuse non-symlink target without --force")
	}
	combined := runErr.Error() + "\n" + errOut
	if !strings.Contains(combined, "refusing to overwrite") {
		t.Errorf("explicit install must name the refusal; got: %s", combined)
	}
	if !strings.Contains(combined, "anvil install skills --force") {
		t.Errorf("explicit refusal must name --force escape; got: %s", combined)
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
