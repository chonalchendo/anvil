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
const sessionEndHookCommand = `anvil session end --commit`

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

// resolveCodexConfigDir returns Codex's config dir: $CODEX_HOME if set, else
// ~/.codex. Mirrors resolveClaudeConfigDir for the second agent CLI; skills
// land under its skills/ subdir.
func resolveCodexConfigDir() (string, error) {
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

// resolveAgentCLIConfigDir picks the agent CLI's config dir for skill/agent
// install. Only "claude" and "codex" are valid; an unknown target is a usage
// error.
func resolveAgentCLIConfigDir(target string) (string, error) {
	switch target {
	case "claude":
		return resolveClaudeConfigDir()
	case "codex":
		return resolveCodexConfigDir()
	default:
		return "", fmt.Errorf("unknown --target %q: want claude or codex", target)
	}
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
				changedStart, err := installer.RemoveSessionStartHook(path, sessionStartHookCommand)
				if err != nil {
					return fmt.Errorf("removing SessionStart hook: %w", err)
				}
				changedStop, err := installer.RemoveStopHook(path, sessionEndHookCommand)
				if err != nil {
					return fmt.Errorf("removing Stop hook: %w", err)
				}
				if changedStart || changedStop {
					cmd.Println("removed anvil hooks from", path)
				} else {
					cmd.Println("no matching anvil hooks in", path)
				}
				return nil
			}
			changedStart, err := installer.MergeSessionStartHook(path, sessionStartHookCommand)
			if err != nil {
				return fmt.Errorf("installing SessionStart hook: %w", err)
			}
			changedStop, err := installer.MergeStopHook(path, sessionEndHookCommand)
			if err != nil {
				return fmt.Errorf("installing Stop hook: %w", err)
			}
			if changedStart || changedStop {
				cmd.Println("installed anvil hooks in", path)
			} else {
				cmd.Println("anvil hooks already installed in", path)
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
	var target string
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install (or remove) the binary-embedded Anvil skills into an agent CLI's skills dir",
		Long: "Install (or remove) the Anvil skills bundle into the target agent CLI's skills dir:\n" +
			"--target claude → ~/.claude/skills/<name>/ (symlinked); --target codex →\n" +
			"~/.codex/skills/<name>/ (copied, honoring $CODEX_HOME). Codex copies because its\n" +
			"skill discovery following a symlinked skill directory is unverified.\n\n" +
			"Skills are embedded into the anvil binary at build time. This command deploys\n" +
			"that embedded bundle — it does NOT read anvil/skills/ from disk. Editing\n" +
			"anvil/skills/<name>/SKILL.md in an anvil checkout has no effect until you rebuild\n" +
			"the binary (`just install`) and re-run `anvil install skills --force`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			skillsDir, err := resolveAnvilSkillsTarget(target)
			if err != nil {
				return err
			}
			mat, err := resolveAnvilSkillsMaterialiseDir()
			if err != nil {
				return err
			}
			if uninstall {
				changed, err := installer.RemoveSkills(skills.FS, mat, skillsDir)
				if err != nil {
					return fmt.Errorf("removing skills: %w", err)
				}
				if changed {
					cmd.Println("removed anvil skills from", skillsDir)
				} else {
					cmd.Println("no anvil skills found at", skillsDir)
				}
				return nil
			}
			if target == "codex" {
				// Codex always copies (symlinked skill dirs unverified) and
				// skips the freshness shortcut below: that shortcut keys off the
				// materialise dir's hash, which a prior `--target claude` install
				// can leave fresh while the separate Codex skills dir holds
				// nothing — skipping would then report "up to date" without ever
				// copying. Reinstalling is idempotent and cheap, so just do it.
				useCopy = true
			} else if !force {
				// Skip the install when on-disk content already matches the
				// embedded bundle and the user didn't pass --force. This makes
				// re-running `anvil install skills` a content-aware no-op rather
				// than a confusing "already installed" wall — the only case where
				// we'd refuse useful work is when the embed has drifted, and the
				// hash check covers that.
				if _, err := os.Stat(mat); err == nil {
					fresh, err := installer.SkillsAreFresh(skills.FS, mat)
					if err != nil {
						return fmt.Errorf("checking skills freshness: %w", err)
					}
					if fresh {
						// Bundle content is current, but orphaned symlinks from
						// removed skills may still exist — prune them even though
						// we skip the full install.
						if _, err := installer.PruneOrphanedSkills(skills.FS, mat, skillsDir); err != nil {
							return fmt.Errorf("pruning orphaned skills: %w", err)
						}
						cmd.Println("anvil skills up to date at", skillsDir+" (embedded bundle); run `anvil install skills --force` to redeploy, or `just install` first if you edited anvil/skills/ on disk")
						return nil
					}
				}
			}
			_, err = installer.InstallSkills(skills.FS, mat, skillsDir, useCopy, force)
			if err != nil {
				return fmt.Errorf("installing skills: %w", err)
			}
			if useCopy {
				cmd.Println("copied anvil skills (embedded bundle) into", skillsDir+"; rebuild with `just install` to refresh after editing anvil/skills/ on disk")
			} else {
				cmd.Println("linked anvil skills (embedded bundle) under", skillsDir, "->", mat+"; rebuild with `just install` to refresh after editing anvil/skills/ on disk")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove anvil skills instead of installing them")
	cmd.Flags().BoolVar(&useCopy, "copy", false, "copy files instead of symlinking (use when symlinks aren't supported)")
	cmd.Flags().BoolVar(&force, "force", false, "redeploy even when installed content matches the embedded bundle")
	cmd.Flags().StringVar(&target, "target", "claude", "agent CLI to install into: claude (~/.claude) or codex (~/.codex, honoring $CODEX_HOME)")
	return cmd
}

func newInstallAgentsCmd() *cobra.Command {
	var uninstall, force bool
	var target string
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Install (or remove) the binary-embedded Anvil agents into an agent CLI's agents dir",
		Long: "Install (or remove) the Anvil agents bundle into the target agent CLI's agents dir:\n" +
			"--target claude → ~/.claude/agents/<name>.md (Claude markdown subagents);\n" +
			"--target codex → ~/.codex/agents/<name>.toml (Codex custom-agent TOML, honoring\n" +
			"$CODEX_HOME). The Codex emit translates each markdown agent — frontmatter\n" +
			"name/description plus the body become the required TOML keys; Claude-specific\n" +
			"model/tools/skills are dropped.\n\n" +
			"Agents are embedded into the anvil binary at build time. This command deploys\n" +
			"that embedded bundle — editing anvil/agents/<name>.md in a checkout has no\n" +
			"effect until you rebuild the binary (`just install`) and re-run\n" +
			"`anvil install agents`. A freshly-deployed Claude agent is dispatchable via the\n" +
			"Agent tool's subagent_type only after the next Claude Code session restart.",
		Example: "  anvil install agents\n  anvil install agents --target codex\n  anvil install agents --target codex --uninstall",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := resolveAnvilAgentsTarget(target)
			if err != nil {
				return err
			}
			if target == "codex" {
				return runInstallCodexAgents(cmd, dir, uninstall, force)
			}
			if uninstall {
				changed, err := installer.RemoveAgents(agents.FS, dir)
				if err != nil {
					return fmt.Errorf("removing agents: %w", err)
				}
				if changed {
					cmd.Println("removed anvil agents from", dir)
				} else {
					cmd.Println("no anvil agents found at", dir)
				}
				return nil
			}
			changed, err := installer.InstallAgents(agents.FS, dir, force)
			if err != nil {
				return fmt.Errorf("installing agents: %w", err)
			}
			if changed {
				cmd.Println("installed anvil agents (embedded bundle) into", dir+"; restart Claude Code before dispatching a freshly-added agent")
			} else {
				cmd.Println("anvil agents up to date at", dir)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove anvil agents instead of installing them")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing agent file that differs from the embedded copy")
	cmd.Flags().StringVar(&target, "target", "claude", "agent CLI to install into: claude (~/.claude, markdown) or codex (~/.codex, TOML; honoring $CODEX_HOME)")
	return cmd
}

func runInstallCodexAgents(cmd *cobra.Command, dir string, uninstall, force bool) error {
	if uninstall {
		changed, err := installer.RemoveCodexAgents(agents.FS, dir)
		if err != nil {
			return fmt.Errorf("removing codex agents: %w", err)
		}
		if changed {
			cmd.Println("removed anvil agents from", dir)
		} else {
			cmd.Println("no anvil agents found at", dir)
		}
		return nil
	}
	changed, err := installer.InstallCodexAgents(agents.FS, dir, force)
	if err != nil {
		return fmt.Errorf("installing codex agents: %w", err)
	}
	if changed {
		cmd.Println("installed anvil agents (embedded bundle) as Codex TOML into", dir)
	} else {
		cmd.Println("anvil agents up to date at", dir)
	}
	return nil
}

// resolveAnvilSkillsTarget returns the user-skills parent directory for the
// given agent CLI target. Anvil installs each shipped skill flat under this
// path (skills/<skill>/SKILL.md) so the agent CLI's user-skill discovery picks
// them up.
func resolveAnvilSkillsTarget(target string) (string, error) {
	dir, err := resolveAgentCLIConfigDir(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "skills"), nil
}

// resolveAnvilAgentsTarget returns the user-agents directory for the given
// agent CLI target. Anvil installs each shipped agent flat under this path
// (target/<name>.{md,toml}) so the CLI's subagent discovery picks them up.
func resolveAnvilAgentsTarget(target string) (string, error) {
	dir, err := resolveAgentCLIConfigDir(target)
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
	// Auto-refresh tracks only the default Claude install; Codex installs are
	// explicit and opt-in, so a stale Codex bundle is refreshed by re-running
	// `anvil install skills --target codex`, not here.
	target, err := resolveAnvilSkillsTarget("claude")
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
