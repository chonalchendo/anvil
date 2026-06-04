package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/agents"
	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/bases"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/installer"
	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/workflows"
)

func newInitCmd() *cobra.Command {
	var installClaude bool
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Scaffold an Anvil vault",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var v *core.Vault
			if len(args) == 1 {
				v = &core.Vault{Root: args[0]}
			} else {
				rv, err := core.ResolveVault()
				if err != nil {
					return err
				}
				v = rv
			}
			if err := v.Scaffold(); err != nil {
				return err
			}
			if err := writeEmbedded(schema.EmbeddedFS, v.SchemasDir()); err != nil {
				return fmt.Errorf("writing schemas: %w", err)
			}
			if err := writeEmbedded(bases.FS, v.BasesDir()); err != nil {
				return fmt.Errorf("writing bases: %w", err)
			}
			if err := writeEmbedded(workflows.FS, v.WorkflowsDir()); err != nil {
				return fmt.Errorf("writing workflows: %w", err)
			}
			if err := v.EnableObsidianCorePlugin("bases"); err != nil {
				return fmt.Errorf("enabling bases plugin: %w", err)
			}
			cmd.Println("vault scaffolded at", v.Root)
			if installClaude {
				if err := installClaudeComponents(cmd); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&installClaude, "install-claude", false, "install embedded skills, agents, and hooks into ~/.claude after scaffolding")
	return cmd
}

// writeEmbedded copies every file at the root of srcFS into destDir under its
// original name. Used to deploy the embedded schema and bases bundles on init.
func writeEmbedded(srcFS fs.FS, destDir string) error {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return err
	}
	for _, e := range entries {
		b, err := fs.ReadFile(srcFS, e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(destDir, e.Name()), b, 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}
	return nil
}

// installClaudeComponents installs the embedded skills, agents, and hooks into
// the Claude config dir (~/.claude, or $CLAUDE_CONFIG_DIR). Mirrors what
// `anvil install skills`, `anvil install agents`, and `anvil install hooks`
// do individually, but called as a single opt-in step from `anvil init`.
func installClaudeComponents(cmd *cobra.Command) error {
	skillsDir, err := resolveAnvilSkillsTarget("claude")
	if err != nil {
		return err
	}
	mat, err := resolveAnvilSkillsMaterialiseDir()
	if err != nil {
		return err
	}
	if _, err := installer.InstallSkills(skills.FS, mat, skillsDir, false, false); err != nil {
		return fmt.Errorf("installing skills: %w", err)
	}
	cmd.Println("installed anvil skills into", skillsDir)

	agentsDir, err := resolveAnvilAgentsTarget("claude")
	if err != nil {
		return err
	}
	if _, err := installer.InstallAgents(agents.FS, agentsDir, false); err != nil {
		return fmt.Errorf("installing agents: %w", err)
	}
	cmd.Println("installed anvil agents into", agentsDir)

	settingsPath, err := resolveClaudeSettingsPath()
	if err != nil {
		return err
	}
	if _, err := installer.MergeSessionStartHook(settingsPath, sessionStartHookCommand); err != nil {
		return fmt.Errorf("installing hooks: %w", err)
	}
	cmd.Println("installed SessionStart hook in", settingsPath)
	return nil
}
