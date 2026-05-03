package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_Hooks_RespectsClaudeConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "hooks"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install hooks: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing: %v", got)
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart hook not written")
	}
	if !strings.Contains(out.String(), "installed") {
		t.Errorf("output = %q, want to mention installed", out.String())
	}
}

func TestInstall_Hooks_FallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "hooks"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install hooks: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err != nil {
		t.Errorf("settings.json missing under HOME/.claude: %v", err)
	}
}

func TestInstall_Hooks_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	for i := 0; i < 2; i++ {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "hooks"})
		cmd.SetOut(&bytes.Buffer{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	b, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Errorf("SessionStart len = %d after 2 installs, want 1", len(ss))
	}
}

func TestInstall_Hooks_Uninstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "hooks"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"install", "hooks", "--uninstall"})
	var out bytes.Buffer
	cmd2.SetOut(&out)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	hooks, _ := got["hooks"].(map[string]any)
	if hooks != nil {
		if ss, ok := hooks["SessionStart"].([]any); ok && len(ss) > 0 {
			t.Errorf("SessionStart still present: %v", ss)
		}
	}
	if !strings.Contains(out.String(), "removed") {
		t.Errorf("output = %q, want to mention removed", out.String())
	}
}
