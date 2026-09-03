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
	// Two anvil-managed entries: the unmatched fire-session-start (every
	// source) and the resume|compact matcher-scoped fire-session-resume.
	// Idempotence means this stays at 2, not that it grows past it.
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 2 {
		t.Errorf("SessionStart len = %d after 2 installs, want 2", len(ss))
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
		fmParts := strings.SplitN(got, "---\n", 3)
		if len(fmParts) < 3 {
			t.Fatalf("emitted markdown missing frontmatter delimiters\n---\n%s", got)
		}
		var fm map[string]any
		if err := yaml.Unmarshal([]byte(fmParts[1]), &fm); err != nil {
			t.Fatalf("emitted frontmatter is not valid strict YAML: %v\n---\n%s", err, fmParts[1])
		}
		want := map[string]any{
			"name":     "anvil-issue-worker",
			"model":    "claude-bridge/claude-sonnet-5",
			"thinking": "medium",
			"skills":   "completing-issue",
		}
		for k, v := range want {
			if fm[k] != v {
				t.Errorf("fm[%q] = %v, want %v\n---\n%s", k, fm[k], v, got)
			}
		}
		if fm["description"] == "" || fm["description"] == nil {
			t.Errorf("fm[%q] empty, want non-empty\n---\n%s", "description", got)
		}
		if _, ok := fm["effort"]; ok {
			t.Errorf("fm should not contain %q, got %v\n---\n%s", "effort", fm["effort"], got)
		}
		for _, unresolvable := range []string{"ToolSearch", "TaskOutput", "TaskStop"} {
			if strings.Contains(fmt.Sprint(fm["tools"]), unresolvable) {
				t.Errorf("tools frontmatter should not contain Claude-only %q, got %v", unresolvable, fm["tools"])
			}
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "agents")); !os.IsNotExist(err) {
			t.Errorf("Claude agents dir should be untouched by --target pi, got err=%v", err)
		}

		reviewerMD, err := os.ReadFile(filepath.Join(piDir, "agents", "anvil-pr-reviewer.md")) //nolint:gosec // path is test-controlled
		if err != nil {
			t.Fatalf("read emitted anvil-pr-reviewer.md: %v", err)
		}
		reviewerParts := strings.SplitN(string(reviewerMD), "---\n", 3)
		if len(reviewerParts) < 3 {
			t.Fatalf("emitted anvil-pr-reviewer.md missing frontmatter delimiters\n---\n%s", reviewerMD)
		}
		var reviewerFM map[string]any
		if err := yaml.Unmarshal([]byte(reviewerParts[1]), &reviewerFM); err != nil {
			t.Fatalf("emitted anvil-pr-reviewer.md frontmatter is not valid strict YAML: %v\n---\n%s", err, reviewerParts[1])
		}
		if !strings.Contains(fmt.Sprint(reviewerFM["tools"]), "find") {
			t.Errorf("anvil-pr-reviewer tools = %v, want Glob translated to find", reviewerFM["tools"])
		}
		if strings.Contains(fmt.Sprint(reviewerFM["tools"]), "glob") {
			t.Errorf("anvil-pr-reviewer tools = %v, should not contain untranslated glob", reviewerFM["tools"])
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

// TestInstallAgentsTargetAnte asserts that `install agents --target ante`
// emits each embedded agent as Ante-compatible markdown under
// $ANTE_HOME/agents, leaves the Claude config dir untouched, and that
// --uninstall removes the emitted file.
func TestInstallAgentsTargetAnte(t *testing.T) {
	t.Run("emits markdown into ANTE_HOME and leaves Claude untouched", func(t *testing.T) {
		anteDir := t.TempDir()
		claudeDir := t.TempDir()
		t.Setenv("ANTE_HOME", anteDir)
		t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "agents", "--target", "ante"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install agents --target ante: %v", err)
		}

		md := filepath.Join(anteDir, "agents", "anvil-issue-worker.md")
		b, err := os.ReadFile(md) //nolint:gosec // path is test-controlled
		if err != nil {
			t.Fatalf("read emitted markdown: %v", err)
		}
		got := string(b)
		fmParts := strings.SplitN(got, "---\n", 3)
		if len(fmParts) < 3 {
			t.Fatalf("emitted markdown missing frontmatter delimiters\n---\n%s", got)
		}
		var fm map[string]any
		if err := yaml.Unmarshal([]byte(fmParts[1]), &fm); err != nil {
			t.Fatalf("emitted frontmatter is not valid strict YAML: %v\n---\n%s", err, fmParts[1])
		}
		if fm["name"] != "anvil-issue-worker" {
			t.Errorf("fm[name] = %v, want anvil-issue-worker\n---\n%s", fm["name"], got)
		}
		if fm["description"] == "" || fm["description"] == nil {
			t.Errorf("fm[description] empty, want non-empty\n---\n%s", got)
		}
		for _, unwanted := range []string{"effort", "skills"} {
			if _, ok := fm[unwanted]; ok {
				t.Errorf("fm should not contain %q, got %v\n---\n%s", unwanted, fm[unwanted], got)
			}
		}
		for _, unresolvable := range []string{"ToolSearch", "TaskOutput", "TaskStop"} {
			if strings.Contains(fmt.Sprint(fm["tools"]), unresolvable) {
				t.Errorf("tools frontmatter should not contain Claude-only %q, got %v", unresolvable, fm["tools"])
			}
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "agents")); !os.IsNotExist(err) {
			t.Errorf("Claude agents dir should be untouched by --target ante, got err=%v", err)
		}
	})

	t.Run("uninstall removes the emitted markdown", func(t *testing.T) {
		anteDir := t.TempDir()
		t.Setenv("ANTE_HOME", anteDir)
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

		install := newRootCmd()
		install.SetArgs([]string{"install", "agents", "--target", "ante"})
		install.SetOut(&bytes.Buffer{})
		if err := install.Execute(); err != nil {
			t.Fatalf("install: %v", err)
		}

		uninstall := newRootCmd()
		uninstall.SetArgs([]string{"install", "agents", "--target", "ante", "--uninstall"})
		var out bytes.Buffer
		uninstall.SetOut(&out)
		if err := uninstall.Execute(); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if _, err := os.Stat(filepath.Join(anteDir, "agents", "anvil-issue-worker.md")); !os.IsNotExist(err) {
			t.Errorf("emitted markdown should be removed, got err=%v", err)
		}
		if !strings.Contains(out.String(), "removed") {
			t.Errorf("output = %q, want mention of removed", out.String())
		}
	})
}

// TestInstallSkillsTargetPi asserts that `install skills --target pi` copies
// the embedded bundle (no symlinks) into $PI_CODING_AGENT_DIR/skills, leaves
// the Claude skills dir untouched, and that --uninstall removes the copy.
func TestInstallSkillsTargetPi(t *testing.T) {
	t.Run("copies skills into PI_CODING_AGENT_DIR and leaves Claude untouched", func(t *testing.T) {
		piDir := t.TempDir()
		claudeDir := t.TempDir()
		t.Setenv("PI_CODING_AGENT_DIR", piDir)
		t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "skills", "--target", "pi"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install skills --target pi: %v", err)
		}

		skillDir := filepath.Join(piDir, "skills", "completing-issue")
		info, err := os.Lstat(skillDir)
		if err != nil {
			t.Fatalf("stat emitted skill dir: %v", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("emitted skill dir should be a real copy, not a symlink")
		}
		if _, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md")); err != nil { //nolint:gosec // path is test-controlled
			t.Fatalf("read copied SKILL.md: %v", err)
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "skills")); !os.IsNotExist(err) {
			t.Errorf("Claude skills dir should be untouched by --target pi, got err=%v", err)
		}
		if !strings.Contains(out.String(), "copied") {
			t.Errorf("output = %q, want mention of copied", out.String())
		}
	})

	t.Run("uninstall removes the copied skills", func(t *testing.T) {
		piDir := t.TempDir()
		t.Setenv("PI_CODING_AGENT_DIR", piDir)
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

		install := newRootCmd()
		install.SetArgs([]string{"install", "skills", "--target", "pi"})
		install.SetOut(&bytes.Buffer{})
		if err := install.Execute(); err != nil {
			t.Fatalf("install: %v", err)
		}

		uninstall := newRootCmd()
		uninstall.SetArgs([]string{"install", "skills", "--target", "pi", "--uninstall"})
		var out bytes.Buffer
		uninstall.SetOut(&out)
		if err := uninstall.Execute(); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if _, err := os.Stat(filepath.Join(piDir, "skills", "completing-issue")); !os.IsNotExist(err) {
			t.Errorf("copied skill dir should be removed, got err=%v", err)
		}
		if !strings.Contains(out.String(), "removed") {
			t.Errorf("output = %q, want mention of removed", out.String())
		}
	})
}

// TestInstallSkillsTargetAnte asserts that `install skills --target ante`
// symlinks the embedded bundle into $ANTE_HOME/skills (same shape as the
// default claude target, since resolveAgentCLIConfigDir/resolveAnvilSkillsTarget
// route ante through the generic path), leaves the Claude skills dir
// untouched, and that --uninstall removes the symlink.
func TestInstallSkillsTargetAnte(t *testing.T) {
	t.Run("symlinks skills into ANTE_HOME and leaves Claude untouched", func(t *testing.T) {
		anteDir := t.TempDir()
		claudeDir := t.TempDir()
		t.Setenv("ANTE_HOME", anteDir)
		t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
		t.Setenv("ANVIL_SKILLS_DIR", t.TempDir())

		cmd := newRootCmd()
		cmd.SetArgs([]string{"install", "skills", "--target", "ante"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install skills --target ante: %v", err)
		}

		skillDir := filepath.Join(anteDir, "skills", "completing-issue")
		info, err := os.Lstat(skillDir)
		if err != nil {
			t.Fatalf("stat emitted skill dir: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("emitted skill dir should be symlinked, like the default claude target")
		}
		if _, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md")); err != nil { //nolint:gosec // path is test-controlled
			t.Fatalf("read linked SKILL.md: %v", err)
		}
		if _, err := os.Stat(filepath.Join(claudeDir, "skills")); !os.IsNotExist(err) {
			t.Errorf("Claude skills dir should be untouched by --target ante, got err=%v", err)
		}
	})

	t.Run("uninstall removes the linked skills", func(t *testing.T) {
		anteDir := t.TempDir()
		t.Setenv("ANTE_HOME", anteDir)
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
		t.Setenv("ANVIL_SKILLS_DIR", t.TempDir())

		install := newRootCmd()
		install.SetArgs([]string{"install", "skills", "--target", "ante"})
		install.SetOut(&bytes.Buffer{})
		if err := install.Execute(); err != nil {
			t.Fatalf("install: %v", err)
		}

		uninstall := newRootCmd()
		uninstall.SetArgs([]string{"install", "skills", "--target", "ante", "--uninstall"})
		var out bytes.Buffer
		uninstall.SetOut(&out)
		if err := uninstall.Execute(); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if _, err := os.Lstat(filepath.Join(anteDir, "skills", "completing-issue")); !os.IsNotExist(err) {
			t.Errorf("linked skill dir should be removed, got err=%v", err)
		}
		if !strings.Contains(out.String(), "removed") {
			t.Errorf("output = %q, want mention of removed", out.String())
		}
	})
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
	if _, exists := got["autoCompactWindow"]; exists {
		t.Errorf("autoCompactWindow still present after uninstall: %v", got["autoCompactWindow"])
	}
}
