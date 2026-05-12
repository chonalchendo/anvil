package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/installer"
	"github.com/chonalchendo/anvil/skills"
)

const sessionStartHookCommand = `anvil install fire-session-start`

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Anvil components into the user environment",
	}
	cmd.AddCommand(newInstallHooksCmd(), newInstallSkillsCmd(), newInstallFireSessionStartCmd())
	return cmd
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
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "settings.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func newInstallSkillsCmd() *cobra.Command {
	var uninstall, useCopy bool
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install (or remove) the bundled Anvil skills into ~/.claude/skills/anvil",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveAnvilSkillsTarget()
			if err != nil {
				return err
			}
			if uninstall {
				changed, err := installer.RemoveSkills(target)
				if err != nil {
					return fmt.Errorf("removing skills: %w", err)
				}
				if changed {
					cmd.Println("removed skills bundle at", target)
				} else {
					cmd.Println("no skills bundle at", target)
				}
				return nil
			}
			mat, err := resolveAnvilSkillsMaterialiseDir()
			if err != nil {
				return err
			}
			changed, err := installer.InstallSkills(skills.FS, mat, target, useCopy)
			if err != nil {
				return fmt.Errorf("installing skills: %w", err)
			}
			switch {
			case changed && useCopy:
				cmd.Println("copied skills bundle to", target)
			case changed:
				cmd.Println("linked skills bundle at", target, "->", mat)
			default:
				cmd.Println("skills bundle already installed at", target)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the bundle instead of installing it")
	cmd.Flags().BoolVar(&useCopy, "copy", false, "copy files instead of symlinking (use when symlinks aren't supported)")
	return cmd
}

func resolveAnvilSkillsTarget() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "skills", "anvil"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "skills", "anvil"), nil
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
