package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/installer"
	"github.com/chonalchendo/anvil/skills"
)

// autoRefreshWarned tracks offending paths the current process has already
// warned about during auto-refresh. Process-scoped only — explicit v0.1
// non-goal to persist across invocations. Keyed by the offending absolute
// path extracted from the installer refusal error.
var autoRefreshWarned sync.Map

// resetAutoRefreshWarnedForTest clears the once-per-process warning gate so
// tests can exercise repeated invocations within a single test process.
func resetAutoRefreshWarnedForTest() {
	autoRefreshWarned.Range(func(k, _ any) bool {
		autoRefreshWarned.Delete(k)
		return true
	})
}

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
	var uninstall, useCopy, force bool
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install (or remove) the bundled Anvil skills into ~/.claude/skills/<name>/",
		Args:  cobra.NoArgs,
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
						cmd.Println("anvil skills up to date at", target+"; run `anvil install skills --force` to redeploy")
						return nil
					}
				}
			}
			_, err = installer.InstallSkills(skills.FS, mat, target, useCopy)
			if err != nil {
				return fmt.Errorf("installing skills: %w", err)
			}
			if useCopy {
				cmd.Println("copied anvil skills into", target)
			} else {
				cmd.Println("linked anvil skills under", target, "->", mat)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove anvil skills instead of installing them")
	cmd.Flags().BoolVar(&useCopy, "copy", false, "copy files instead of symlinking (use when symlinks aren't supported)")
	cmd.Flags().BoolVar(&force, "force", false, "redeploy even when installed content matches the embedded bundle")
	return cmd
}

// resolveAnvilSkillsTarget returns the user-skills parent directory. Anvil
// installs each shipped skill flat under this path (target/<skill>/SKILL.md)
// so Claude Code's user-skill discovery picks them up.
func resolveAnvilSkillsTarget() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "skills"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "skills"), nil
}

// refreshSkillsIfStale auto-rebuilds the installed skills bundle when its
// content diverges from the binary's embedded skills (e.g. after `go install`
// rebuilt the binary). It is a no-op when skills were never installed, or
// when the current command is itself an install subcommand. Failures are
// logged to stderr but never abort the command.
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
		// Suppress repeat warnings for the same offending path within this
		// process. The path is embedded in the installer refusal error
		// (format locked by T1 in internal/installer/skills.go). For errors
		// we can't classify, fall through and warn — better noisy than
		// silently dropping a new failure mode.
		key := refusalPathKey(err)
		if _, loaded := autoRefreshWarned.LoadOrStore(key, struct{}{}); loaded {
			return
		}
		cmd.PrintErrln("anvil: skills auto-refresh failed:", err)
		return
	}
	if refreshed {
		cmd.PrintErrln("anvil: refreshed stale skills bundle at", target)
	}
}

// refusalPathKey extracts the offending path from an installer refusal
// error so once-per-process warning suppression keys on the path, not the
// error identity. Returns the raw error string when the refusal marker is
// absent — that bucket warns at most once for non-refusal failure modes.
func refusalPathKey(err error) string {
	msg := err.Error()
	marker := "refusing to overwrite"
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return msg
	}
	rest := strings.TrimLeft(msg[idx+len(marker):], " ")
	for _, prefix := range []string{"non-symlink ", "non-anvil dir "} {
		if strings.HasPrefix(rest, prefix) {
			rest = rest[len(prefix):]
			if end := strings.Index(rest, ";"); end >= 0 {
				return strings.TrimSpace(rest[:end])
			}
			return strings.TrimSpace(rest)
		}
	}
	return msg
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
