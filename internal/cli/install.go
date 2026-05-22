package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/agents"
	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/installer"
)

const sessionStartHookCommand = `anvil install fire-session-start`

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Anvil components into the user environment",
	}
	cmd.AddCommand(newInstallHooksCmd(), newInstallSkillsCmd(), newInstallAgentsCmd(), newInstallFireSessionStartCmd())
	return cmd
}

// resolveClaudeConfigDir returns the tool config dir anvil deploys into:
// $CLAUDE_CONFIG_DIR if set, else ~/.claude. Each component type appends its
// own subdir (settings.json, skills/, agents/). A second tool would resolve a
// different config dir here; the per-component layout below is shared.
func resolveClaudeConfigDir() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

func newInstallHooksCmd() *cobra.Command {
	var uninstall bool
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Install (or remove) the Claude Code SessionStart hook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveClaudeSettingsPath()
			if err != nil {
				return err
			}
			if uninstall {
				changed, err := installer.RemoveSessionStartHook(path, sessionStartHookCommand)
				if err != nil {
					return fmt.Errorf("removing hook: %w", err)
				}
				if changed {
					cmd.Println("removed SessionStart hook from", path)
				} else {
					cmd.Println("no matching SessionStart hook in", path)
				}
				return nil
			}
			changed, err := installer.MergeSessionStartHook(path, sessionStartHookCommand)
			if err != nil {
				return fmt.Errorf("installing hook: %w", err)
			}
			if changed {
				cmd.Println("installed SessionStart hook in", path)
			} else {
				cmd.Println("SessionStart hook already installed in", path)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the hook instead of installing it")
	return cmd
}

func resolveClaudeSettingsPath() (string, error) {
	dir, err := resolveClaudeConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func newInstallSkillsCmd() *cobra.Command {
	var uninstall, useCopy, force bool
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install (or remove) the binary-embedded Anvil skills into ~/.claude/skills/<name>/",
		Long: "Install (or remove) the Anvil skills bundle into ~/.claude/skills/<name>/.\n\n" +
			"Skills are embedded into the anvil binary at build time. This command deploys\n" +
			"that embedded bundle — it does NOT read anvil/skills/ from disk. Editing\n" +
			"anvil/skills/<name>/SKILL.md in an anvil checkout has no effect until you rebuild\n" +
			"the binary (`just install`) and re-run `anvil install skills --force`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveAnvilSkillsTarget()
			if err != nil {
				return err
			}
			mat, err := resolveAnvilSkillsMaterialiseDir()
			if err != nil {
				return err
			}
			if uninstall {
				changed, err := installer.RemoveSkills(skills.FS, mat, target)
				if err != nil {
					return fmt.Errorf("removing skills: %w", err)
				}
				if changed {
					cmd.Println("removed anvil skills from", target)
				} else {
					cmd.Println("no anvil skills found at", target)
				}
				return nil
			}
			// Skip the install when on-disk content already matches the
			// embedded bundle and the user didn't pass --force. This makes
			// re-running `anvil install skills` a content-aware no-op rather
			// than a confusing "already installed" wall — the only case where
			// we'd refuse useful work is when the embed has drifted, and the
			// hash check covers that.
			if !force {
				if _, err := os.Stat(mat); err == nil {
					fresh, err := installer.SkillsAreFresh(skills.FS, mat)
					if err != nil {
						return fmt.Errorf("checking skills freshness: %w", err)
					}
					if fresh {
						cmd.Println("anvil skills up to date at", target+" (embedded bundle); run `anvil install skills --force` to redeploy, or `just install` first if you edited skills/ on disk")
						return nil
					}
				}
			}
			_, err = installer.InstallSkills(skills.FS, mat, target, useCopy, force)
			if err != nil {
				return fmt.Errorf("installing skills: %w", err)
			}
			if useCopy {
				cmd.Println("copied anvil skills (embedded bundle) into", target+"; rebuild with `just install` to refresh after editing anvil/skills/ on disk")
			} else {
				cmd.Println("linked anvil skills (embedded bundle) under", target, "->", mat+"; rebuild with `just install` to refresh after editing anvil/skills/ on disk")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove anvil skills instead of installing them")
	cmd.Flags().BoolVar(&useCopy, "copy", false, "copy files instead of symlinking (use when symlinks aren't supported)")
	cmd.Flags().BoolVar(&force, "force", false, "redeploy even when installed content matches the embedded bundle")
	return cmd
}

func newInstallAgentsCmd() *cobra.Command {
	var uninstall, force bool
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Install (or remove) the binary-embedded Anvil agents into ~/.claude/agents/<name>.md",
		Long: "Install (or remove) the Anvil agents bundle into ~/.claude/agents/<name>.md.\n\n" +
			"Agents are embedded into the anvil binary at build time. This command deploys\n" +
			"that embedded bundle — editing anvil/agents/<name>.md in a checkout has no\n" +
			"effect until you rebuild the binary (`just install`) and re-run\n" +
			"`anvil install agents`. A freshly-deployed agent is dispatchable via the Agent\n" +
			"tool's subagent_type only after the next Claude Code session restart.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveAnvilAgentsTarget()
			if err != nil {
				return err
			}
			if uninstall {
				changed, err := installer.RemoveAgents(agents.FS, target)
				if err != nil {
					return fmt.Errorf("removing agents: %w", err)
				}
				if changed {
					cmd.Println("removed anvil agents from", target)
				} else {
					cmd.Println("no anvil agents found at", target)
				}
				return nil
			}
			changed, err := installer.InstallAgents(agents.FS, target, force)
			if err != nil {
				return fmt.Errorf("installing agents: %w", err)
			}
			if changed {
				cmd.Println("installed anvil agents (embedded bundle) into", target+"; restart Claude Code before dispatching a freshly-added agent")
			} else {
				cmd.Println("anvil agents up to date at", target)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove anvil agents instead of installing them")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing agent file that differs from the embedded copy")
	return cmd
}

// resolveAnvilSkillsTarget returns the user-skills parent directory. Anvil
// installs each shipped skill flat under this path (target/<skill>/SKILL.md)
// so Claude Code's user-skill discovery picks them up.
func resolveAnvilSkillsTarget() (string, error) {
	dir, err := resolveClaudeConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "skills"), nil
}

// resolveAnvilAgentsTarget returns the user-agents directory. Anvil installs
// each shipped agent flat under this path (target/<name>.md) so Claude Code's
// subagent discovery picks them up.
func resolveAnvilAgentsTarget() (string, error) {
	dir, err := resolveClaudeConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agents"), nil
}

// refreshSkillsIfStale auto-rebuilds the installed skills bundle when its
// content diverges from the binary's embedded skills (e.g. after `go install`
// rebuilt the binary). It is a no-op when skills were never installed, or
// when the current command is itself an install subcommand, or when the
// refresh would clobber a user-managed non-symlink target (installer
// swallows that case — the explicit `anvil install skills` is where users
// expect to be told about the conflict). Other failures are surfaced to
// stderr but never abort the command.
func refreshSkillsIfStale(cmd *cobra.Command) {
	if strings.HasPrefix(cmd.CommandPath(), "anvil install") {
		return
	}
	mat, err := resolveAnvilSkillsMaterialiseDir()
	if err != nil {
		return
	}
	target, err := resolveAnvilSkillsTarget()
	if err != nil {
		return
	}
	refreshed, err := installer.RefreshSkillsIfStale(skills.FS, mat, target)
	if err != nil {
		cmd.PrintErrln("anvil: skills auto-refresh failed:", err)
		return
	}
	if refreshed {
		cmd.PrintErrln("anvil: refreshed stale skills bundle at", target)
	}
}

func resolveAnvilSkillsMaterialiseDir() (string, error) {
	if d := os.Getenv("ANVIL_SKILLS_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".anvil", "skills"), nil
}
