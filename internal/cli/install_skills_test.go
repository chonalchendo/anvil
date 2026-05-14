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
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, ".anvil-skills-hash"), []byte("old"), 0o644); err != nil {
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
