package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoot_AutoRefreshesStaleSkills verifies that auto-refresh fires when the
// deploy stamp is absent (backward-compatible path: pre-version-guard install).
// The stamp is removed after install to simulate an install done by an older
// binary that did not write the stamp; a stale hash then triggers the fallback
// hash-only logic and the bundle is rewritten.
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

	// Remove the deploy stamp to simulate a pre-version-guard install so the
	// backward-compatible hash-only path is exercised.
	stampPath := filepath.Join(skillsRoot, ".anvil-skills-deploy-stamp")
	if err := os.Remove(stampPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stamp: %v", err)
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

// TestRoot_SkipsAutoRefreshForForeignDeploy confirms that when the deploy stamp
// is present and the binary has not been rebuilt (stamp >= binary mtime), an
// unrelated command does not overwrite the installed bundle even when the hash
// file records a divergent value — protecting against a stale global binary
// silently downgrading skills deployed by a newer worktree binary.
func TestRoot_SkipsAutoRefreshForForeignDeploy(t *testing.T) {
	skillsRoot := t.TempDir()
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)
	setupVault(t)

	if _, _, err := runCmd(t, newRootCmd(), "install", "skills"); err != nil {
		t.Fatalf("install skills: %v", err)
	}

	// Add a marker to a materialised skill to detect if auto-refresh fires.
	// Find the first SKILL.md in the materialise dir.
	var skillmd string
	if entries, rerr := os.ReadDir(skillsRoot); rerr == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(skillsRoot, e.Name(), "SKILL.md")
			if _, serr := os.Stat(candidate); serr == nil {
				skillmd = candidate
				break
			}
		}
	}
	if skillmd == "" {
		t.Fatal("no SKILL.md found in materialise dir")
	}
	f, err := os.OpenFile(skillmd, os.O_APPEND|os.O_WRONLY, 0o644) //nolint:gosec // path is test-controlled
	if err != nil {
		t.Fatalf("open skill: %v", err)
	}
	_, werr := f.WriteString("\nMARKER_FOREIGN_DEPLOY\n")
	if cerr := f.Close(); cerr != nil && werr == nil {
		werr = cerr
	}
	if werr != nil {
		t.Fatalf("write marker: %v", werr)
	}

	// Simulate a divergent hash (as if a different binary deployed skills).
	hashPath := filepath.Join(skillsRoot, ".anvil-skills-hash")
	if err := os.WriteFile(hashPath, []byte("deadbeefdeadbeef"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatalf("write foreign hash: %v", err)
	}

	_, _, err = runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("where: %v", err)
	}

	data, err := os.ReadFile(skillmd) //nolint:gosec // path is test-controlled
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "MARKER_FOREIGN_DEPLOY") {
		t.Error("auto-refresh overwrote skills despite stamp guard; marker lost")
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
