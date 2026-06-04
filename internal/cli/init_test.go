package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// TestInit_InstallClaude_FlagInstallsComponents verifies that --install-claude
// materialises skills and agents under $CLAUDE_CONFIG_DIR after scaffolding,
// matching the indirect verification predicate in the issue.
func TestInit_InstallClaude_FlagInstallsComponents(t *testing.T) {
	vaultDir := t.TempDir()
	claudeDir := t.TempDir()
	skillsDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("ANVIL_SKILLS_DIR", skillsDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"init", vaultDir, "--install-claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --install-claude: %v", err)
	}

	// Skills must be present under $CLAUDE_CONFIG_DIR/skills/
	skillsTarget := filepath.Join(claudeDir, "skills")
	entries, err := os.ReadDir(skillsTarget)
	if err != nil {
		t.Fatalf("reading skills dir %s: %v", skillsTarget, err)
	}
	if len(entries) == 0 {
		t.Errorf("expected skills in %s, got none", skillsTarget)
	}

	// Agents must be present under $CLAUDE_CONFIG_DIR/agents/
	agentsTarget := filepath.Join(claudeDir, "agents")
	agentEntries, err := os.ReadDir(agentsTarget)
	if err != nil {
		t.Fatalf("reading agents dir %s: %v", agentsTarget, err)
	}
	if len(agentEntries) == 0 {
		t.Errorf("expected agents in %s, got none", agentsTarget)
	}
}

func TestInit_CreatesAllVaultDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANVIL_VAULT", dir)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, d := range core.VaultDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("missing %s: %v", d, err)
		}
	}
}

func TestInit_PathArg(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "custom")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"init", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "00-inbox")); err != nil {
		t.Errorf("expected vault at %s", dir)
	}
}

func TestInit_WritesSchemasIntoVault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANVIL_VAULT", dir)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"inbox", "issue", "plan", "milestone", "decision", "product-design", "system-design"} {
		p := filepath.Join(dir, "schemas", n+".schema.json")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s", p)
		}
	}
}
