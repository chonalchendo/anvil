package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstall_Skills_FlatPerSkillSymlinks confirms each shipped skill lands at
// ~/.claude/skills/<name>/ as a symlink into materialiseDir — the flat layout
// required by Claude Code's user-skill discovery.
func TestInstall_Skills_FlatPerSkillSymlinks(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "skills"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install skills: %v", err)
	}

	target := filepath.Join(claudeDir, "skills")
	child := filepath.Join(target, "capturing-inbox")
	info, err := os.Lstat(child)
	if err != nil {
		t.Fatalf("lstat %s: %v", child, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s should be a symlink, mode=%v", child, info.Mode())
	}
	if got, _ := os.Readlink(child); got != filepath.Join(skillsDir, "capturing-inbox") {
		t.Errorf("symlink = %q, want %q", got, filepath.Join(skillsDir, "capturing-inbox"))
	}
	if _, err := os.Stat(filepath.Join(child, "SKILL.md")); err != nil {
		t.Errorf("capturing-inbox SKILL.md not reachable: %v", err)
	}
	if !strings.Contains(out.String(), "linked anvil skills") {
		t.Errorf("output = %q, want mention of linked anvil skills", out.String())
	}
}

func TestInstall_Skills_Idempotent(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	for i := 0; i < 2; i++ {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "skills"})
		cmd.SetOut(&bytes.Buffer{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if _, err := os.Stat(filepath.Join(claudeDir, "skills", "writing-issue", "SKILL.md")); err != nil {
		t.Errorf("writing-issue not present after 2 installs: %v", err)
	}
}

func TestInstall_Skills_CopyMode(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "skills", "--copy"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install skills --copy: %v", err)
	}
	child := filepath.Join(claudeDir, "skills", "capturing-inbox")
	info, err := os.Lstat(child)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("--copy per-skill target should not be a symlink")
	}
	if !strings.Contains(out.String(), "copied anvil skills") {
		t.Errorf("output = %q, want mention of copied anvil skills", out.String())
	}
}

// TestInstall_Skills_CleansUpLegacyNestedInstall asserts that a prior nested
// install at ~/.claude/skills/anvil/ is removed when a fresh install runs,
// so users upgrading from an earlier anvil version don't end up with two
// copies of every skill on disk.
func TestInstall_Skills_CleansUpLegacyNestedInstall(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	legacy := filepath.Join(claudeDir, "skills", "anvil")
	if err := os.MkdirAll(legacy, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, ".anvil-skills-hash"), []byte("old"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "skills"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install skills: %v", err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy ~/.claude/skills/anvil/ should be removed: %v", err)
	}
}

// TestInstall_Skills_ReinstallReportsUpToDate confirms that re-running
// `anvil install skills` against a vault whose embedded bundle is unchanged
// exits 0, names the next command (`--force`) in the message, and does not
// repeat the "linked" / "copied" wording reserved for actual work.
func TestInstall_Skills_ReinstallReportsUpToDate(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	first := newRootCmd()
	first.SetArgs([]string{"install", "skills"})
	first.SetOut(&bytes.Buffer{})
	if err := first.Execute(); err != nil {
		t.Fatalf("first install: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"install", "skills"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second install: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "up to date") {
		t.Errorf("output = %q, want mention of up to date", got)
	}
	if !strings.Contains(got, "--force") {
		t.Errorf("output = %q, want next-command hint mentioning --force", got)
	}
}

// TestInstall_Skills_ForceRedeploys covers the explicit-overwrite path: with
// --force on an up-to-date install we still rewrite and report linked/copied.
func TestInstall_Skills_ForceRedeploys(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	first := newRootCmd()
	first.SetArgs([]string{"install", "skills"})
	first.SetOut(&bytes.Buffer{})
	if err := first.Execute(); err != nil {
		t.Fatalf("first install: %v", err)
	}

	forced := newRootCmd()
	forced.SetArgs([]string{"install", "skills", "--force"})
	var out bytes.Buffer
	forced.SetOut(&out)
	if err := forced.Execute(); err != nil {
		t.Fatalf("forced install: %v", err)
	}
	if !strings.Contains(out.String(), "linked anvil skills") {
		t.Errorf("output = %q, want linked anvil skills after --force", out.String())
	}
}

// TestInstall_Skills_ForceOverwritesForeignDir confirms `anvil install skills
// --force` does what its flag name promises: a foreign non-anvil directory at
// the shipped name is replaced, not refused with a hint that contradicts the
// invocation. Pins the bug fixed by issue
// anvil-install-skills-force-error-hint-contradicts-the-invoca.
func TestInstall_Skills_ForceOverwritesForeignDir(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	foreign := filepath.Join(claudeDir, "skills", "capturing-inbox")
	if err := os.MkdirAll(foreign, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "user.md"), []byte("user"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "skills", "--force"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install --force: %v\nstderr: %s", err, errOut.String())
	}
	info, err := os.Lstat(foreign)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("--force should leave a symlink at the shipped name; mode=%v", info.Mode())
	}
	if !strings.Contains(out.String(), "linked anvil skills") {
		t.Errorf("output = %q, want linked anvil skills after --force", out.String())
	}
}

// TestInstall_Skills_RefreshesOnContentDrift covers the dogfood case the
// originating issue called out: an installed bundle whose recorded hash is
// stale (e.g. binary rebuilt with new skill bodies) must redeploy automatically
// without --force.
func TestInstall_Skills_RefreshesOnContentDrift(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	first := newRootCmd()
	first.SetArgs([]string{"install", "skills"})
	first.SetOut(&bytes.Buffer{})
	if err := first.Execute(); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Simulate a rebuilt binary whose embed differs from the recorded hash.
	if err := os.WriteFile(filepath.Join(skillsDir, ".anvil-skills-hash"), []byte("stale"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	second := newRootCmd()
	second.SetArgs([]string{"install", "skills"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(out.String(), "linked anvil skills") {
		t.Errorf("output = %q, want linked anvil skills on drift", out.String())
	}
}

func TestInstall_Skills_Uninstall(t *testing.T) {
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "skills"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"install", "skills", "--uninstall"})
	var out bytes.Buffer
	cmd2.SetOut(&out)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(claudeDir, "skills", "capturing-inbox")); !os.IsNotExist(err) {
		t.Errorf("per-skill target should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "capturing-inbox", "SKILL.md")); err != nil {
		t.Errorf("materialised dir should be preserved: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Errorf("output = %q, want mention of removed", out.String())
	}
}

// TestInstallSkillsTargetCodex covers `anvil install skills --target codex`:
// skills land copied (not symlinked) under $CODEX_HOME/skills, the Claude
// config dir is left untouched, the copy still happens when a prior Claude
// install left the materialise dir fresh, and an unknown target is rejected.
func TestInstallSkillsTargetCodex(t *testing.T) {
	t.Run("copies into CODEX_HOME and leaves Claude untouched", func(t *testing.T) {
		codexDir := t.TempDir()
		claudeDir := t.TempDir()
		t.Setenv("CODEX_HOME", codexDir)
		t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
		t.Setenv("ANVIL_SKILLS_DIR", t.TempDir())

		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "skills", "--target", "codex"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install skills --target codex: %v", err)
		}

		child := filepath.Join(codexDir, "skills", "completing-issue")
		info, err := os.Lstat(child)
		if err != nil {
			t.Fatalf("lstat %s: %v", child, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s should be a copy, not a symlink", child)
		}
		if _, err := os.Stat(filepath.Join(child, "SKILL.md")); err != nil {
			t.Errorf("completing-issue SKILL.md not present under CODEX_HOME: %v", err)
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "skills")); !os.IsNotExist(err) {
			t.Errorf("Claude skills dir should be untouched by --target codex, got err=%v", err)
		}
		if !strings.Contains(out.String(), "copied anvil skills") {
			t.Errorf("output = %q, want mention of copied anvil skills", out.String())
		}
	})

	t.Run("copies even when a prior Claude install left the bundle fresh", func(t *testing.T) {
		codexDir := t.TempDir()
		t.Setenv("CODEX_HOME", codexDir)
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
		t.Setenv("ANVIL_SKILLS_DIR", t.TempDir())

		// Prime the materialise dir via a Claude install so its freshness hash
		// is current — the shortcut the codex path must not take.
		claude := newRootCmd()
		claude.SetArgs([]string{"install", "skills"})
		claude.SetOut(&bytes.Buffer{})
		if err := claude.Execute(); err != nil {
			t.Fatalf("seed claude install: %v", err)
		}

		codex := newRootCmd()
		codex.SetArgs([]string{"install", "skills", "--target", "codex"})
		codex.SetOut(&bytes.Buffer{})
		if err := codex.Execute(); err != nil {
			t.Fatalf("install skills --target codex: %v", err)
		}
		if _, err := os.Stat(filepath.Join(codexDir, "skills", "completing-issue", "SKILL.md")); err != nil {
			t.Errorf("codex skills must be copied despite a fresh materialise dir: %v", err)
		}
	})

	t.Run("rejects unknown target", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
		t.Setenv("ANVIL_SKILLS_DIR", t.TempDir())
		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "skills", "--target", "gemini"})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Fatal("want error for unknown --target, got nil")
		}
	})
}
