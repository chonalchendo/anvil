package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

	b, err := os.ReadFile(filepath.Join(dir, "settings.json")) //nolint:gosec // path is test-controlled or application-managed; not user input
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
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
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

	b, _ := os.ReadFile(filepath.Join(dir, "settings.json")) //nolint:gosec // path is test-controlled or application-managed; not user input
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Errorf("SessionStart len = %d after 2 installs, want 1", len(ss))
	}
}

// TestInstallAgentsTargetCodex asserts that `install agents --target codex`
// emits each embedded agent as Codex TOML under $CODEX_HOME/agents, leaves the
// Claude config dir untouched, rejects an unknown target, and that --uninstall
// removes the emitted file.
func TestInstallAgentsTargetCodex(t *testing.T) {
	t.Run("emits TOML into CODEX_HOME and leaves Claude untouched", func(t *testing.T) {
		codexDir := t.TempDir()
		claudeDir := t.TempDir()
		t.Setenv("CODEX_HOME", codexDir)
		t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "agents", "--target", "codex"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install agents --target codex: %v", err)
		}

		toml := filepath.Join(codexDir, "agents", "anvil-issue-worker.toml")
		b, err := os.ReadFile(toml) //nolint:gosec // path is test-controlled
		if err != nil {
			t.Fatalf("read emitted TOML: %v", err)
		}
		got := string(b)
		for _, want := range []string{
			`name = "anvil-issue-worker"`,
			"description = \"",
			"developer_instructions = \"\"\"",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("emitted TOML missing %q\n---\n%s", want, got)
			}
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "agents")); !os.IsNotExist(err) {
			t.Errorf("Claude agents dir should be untouched by --target codex, got err=%v", err)
		}
		if !strings.Contains(out.String(), "Codex TOML") {
			t.Errorf("output = %q, want mention of Codex TOML", out.String())
		}
	})

	t.Run("rejects unknown target", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "agents", "--target", "gemini"})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Fatal("want error for unknown --target, got nil")
		}
	})

	t.Run("uninstall removes the emitted TOML", func(t *testing.T) {
		codexDir := t.TempDir()
		t.Setenv("CODEX_HOME", codexDir)
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

		install := newRootCmd()
		install.SetArgs([]string{"install", "agents", "--target", "codex"})
		install.SetOut(&bytes.Buffer{})
		if err := install.Execute(); err != nil {
			t.Fatalf("install: %v", err)
		}

		uninstall := newRootCmd()
		uninstall.SetArgs([]string{"install", "agents", "--target", "codex", "--uninstall"})
		var out bytes.Buffer
		uninstall.SetOut(&out)
		if err := uninstall.Execute(); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if _, err := os.Stat(filepath.Join(codexDir, "agents", "anvil-issue-worker.toml")); !os.IsNotExist(err) {
			t.Errorf("emitted TOML should be removed, got err=%v", err)
		}
		if !strings.Contains(out.String(), "removed") {
			t.Errorf("output = %q, want mention of removed", out.String())
		}
	})
}

// TestInstallAgentsTargetPi asserts that `install agents --target pi` emits
// each embedded agent as pi-compatible markdown under
// $PI_CODING_AGENT_DIR/agents, leaves the Claude config dir untouched, and
// that --uninstall removes the emitted file.
func TestInstallAgentsTargetPi(t *testing.T) {
	t.Run("emits markdown into PI_CODING_AGENT_DIR and leaves Claude untouched", func(t *testing.T) {
		piDir := t.TempDir()
		claudeDir := t.TempDir()
		t.Setenv("PI_CODING_AGENT_DIR", piDir)
		t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "agents", "--target", "pi"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install agents --target pi: %v", err)
		}

		md := filepath.Join(piDir, "agents", "anvil-issue-worker.md")
		b, err := os.ReadFile(md) //nolint:gosec // path is test-controlled
		if err != nil {
			t.Fatalf("read emitted markdown: %v", err)
		}
		got := string(b)
		for _, want := range []string{"name: anvil-issue-worker", `description: "`, "model: claude-bridge/claude-sonnet-5", "thinking: medium", "skills: completing-issue"} {
			if !strings.Contains(got, want) {
				t.Errorf("emitted markdown missing %q\n---\n%s", want, got)
			}
		}
		if strings.Contains(got, "effort:") {
			t.Errorf("emitted markdown should not contain %q\n---\n%s", "effort:", got)
		}

		fmParts := strings.SplitN(got, "---\n", 3)
		if len(fmParts) < 3 {
			t.Fatalf("emitted markdown missing frontmatter delimiters\n---\n%s", got)
		}
		var fm map[string]any
		if err := yaml.Unmarshal([]byte(fmParts[1]), &fm); err != nil {
			t.Fatalf("emitted frontmatter is not valid strict YAML: %v\n---\n%s", err, fmParts[1])
		}
		for _, unresolvable := range []string{"ToolSearch", "TaskOutput", "TaskStop"} {
			if strings.Contains(fmt.Sprint(fm["tools"]), unresolvable) {
				t.Errorf("tools frontmatter should not contain Claude-only %q, got %v", unresolvable, fm["tools"])
			}
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "agents")); !os.IsNotExist(err) {
			t.Errorf("Claude agents dir should be untouched by --target pi, got err=%v", err)
		}
	})

	t.Run("uninstall removes the emitted markdown", func(t *testing.T) {
		piDir := t.TempDir()
		t.Setenv("PI_CODING_AGENT_DIR", piDir)
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

		install := newRootCmd()
		install.SetArgs([]string{"install", "agents", "--target", "pi"})
		install.SetOut(&bytes.Buffer{})
		if err := install.Execute(); err != nil {
			t.Fatalf("install: %v", err)
		}

		uninstall := newRootCmd()
		uninstall.SetArgs([]string{"install", "agents", "--target", "pi", "--uninstall"})
		var out bytes.Buffer
		uninstall.SetOut(&out)
		if err := uninstall.Execute(); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if _, err := os.Stat(filepath.Join(piDir, "agents", "anvil-issue-worker.md")); !os.IsNotExist(err) {
			t.Errorf("emitted markdown should be removed, got err=%v", err)
		}
		if !strings.Contains(out.String(), "removed") {
			t.Errorf("output = %q, want mention of removed", out.String())
		}
	})
}

func TestInstallSkillsTargetPiRefused(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "skills", "--target", "pi"})
	cmd.SetOut(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --target pi to be refused for install skills")
	}
	if !strings.Contains(err.Error(), "not supported for skills") {
		t.Errorf("error = %q, want mention of unsupported skills target", err.Error())
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

	b, _ := os.ReadFile(filepath.Join(dir, "settings.json")) //nolint:gosec // path is test-controlled or application-managed; not user input
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
