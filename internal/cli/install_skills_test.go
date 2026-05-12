package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_Skills_SymlinkLayout(t *testing.T) {
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

	target := filepath.Join(claudeDir, "skills", "anvil")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target should be a symlink, mode=%v", info.Mode())
	}
	got, _ := os.Readlink(target)
	if got != skillsDir {
		t.Errorf("symlink = %q, want %q", got, skillsDir)
	}
	if _, err := os.Stat(filepath.Join(target, "capturing-inbox", "SKILL.md")); err != nil {
		t.Errorf("capturing-inbox skill not reachable through target: %v", err)
	}
	if !strings.Contains(out.String(), "linked") {
		t.Errorf("output = %q, want mention of linked", out.String())
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
	target := filepath.Join(claudeDir, "skills", "anvil")
	if _, err := os.Stat(filepath.Join(target, "writing-issue", "SKILL.md")); err != nil {
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
	target := filepath.Join(claudeDir, "skills", "anvil")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("--copy target should not be a symlink")
	}
	if !strings.Contains(out.String(), "copied") {
		t.Errorf("output = %q, want mention of copied", out.String())
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
	target := filepath.Join(claudeDir, "skills", "anvil")
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("target should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "capturing-inbox", "SKILL.md")); err != nil {
		t.Errorf("materialised dir should be preserved: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Errorf("output = %q, want mention of removed", out.String())
	}
}
