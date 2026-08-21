package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
	"github.com/chonalchendo/anvil/internal/installer"
)

// cloudSessionEnv is set to "true" by Claude Code inside a cloud session VM and
// is never "true" locally, so it is the gate that keeps this verb inert on a
// developer machine that `anvil install` has already provisioned.
const cloudSessionEnv = "CLAUDE_CODE_REMOTE"

// vaultRemoteMarker identifies the attached vault checkout among the session's
// clones. A cloud session clones each attached repo to a path it chooses, so the
// vault is found by its git remote rather than by a fixed location.
const vaultRemoteMarker = "anvil-vault"

type cloudProvisionReport struct {
	Provisioned bool   `json:"provisioned"`
	Reason      string `json:"reason,omitempty"`
	Vault       string `json:"vault,omitempty"`
	Skills      string `json:"skills,omitempty"`
	Hooks       string `json:"hooks,omitempty"`
	Artifacts   int    `json:"artifacts,omitempty"`
}

func newInstallCloudCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Provision anvil inside a Claude Code cloud session (inert outside one)",
		Long: `Provision anvil inside a Claude Code cloud session.

A cloud session is a fresh VM holding nothing but the attached repo clones:
the skills bundle, the session hooks and the vault all live in the operator's
home directory and never leave their machine. This verb stands that state up
from the binary alone — link the attached vault clone onto the default vault
path, deploy the embedded skills bundle, merge the session hooks, and build the
index a fresh clone does not carry (.anvil/ is gitignored).

Wire it into a repo's .claude/settings.json as a SessionStart hook. Outside a
cloud session it exits 0 having changed nothing, so the same committed hook is
inert on a developer machine.`,
		Example: `  anvil install cloud
  anvil install cloud --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := provisionCloudSession()
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
			}
			if !report.Provisioned {
				cmd.Println("not a cloud session (" + cloudSessionEnv + " unset); nothing to provision")
				return nil
			}
			cmd.Println("provisioned cloud session: vault", report.Vault+",", report.Skills+",", report.Hooks+",", fmt.Sprint(report.Artifacts), "artifacts indexed")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the provisioning report as JSON")
	return cmd
}

func provisionCloudSession() (cloudProvisionReport, error) {
	if os.Getenv(cloudSessionEnv) != "true" {
		return cloudProvisionReport{Reason: "not a cloud session"}, nil
	}

	vaultRoot, err := bindCloudVault()
	if err != nil {
		return cloudProvisionReport{}, err
	}
	report := cloudProvisionReport{Provisioned: true, Vault: vaultRoot}

	skillsDir, err := resolveAnvilSkillsTarget("claude")
	if err != nil {
		return report, err
	}
	mat, err := resolveSkillsMaterialiseDir("claude")
	if err != nil {
		return report, err
	}
	if _, err := installer.InstallSkills(skills.FS, mat, skillsDir, false, true); err != nil {
		return report, fmt.Errorf("installing skills: %w", err)
	}
	report.Skills = skillsDir

	settings, err := resolveClaudeSettingsPath()
	if err != nil {
		return report, err
	}
	if _, err := installer.MergeSessionStartHook(settings, sessionStartHookCommand); err != nil {
		return report, fmt.Errorf("installing SessionStart hook: %w", err)
	}
	if _, err := installer.MergeSessionEndHook(settings, sessionEndHookCommand); err != nil {
		return report, fmt.Errorf("installing SessionEnd hook: %w", err)
	}
	report.Hooks = settings

	v, err := core.ResolveVault()
	if err != nil {
		return report, fmt.Errorf("resolving vault: %w", err)
	}
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		return report, err
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	stats, err := db.Reindex(v.Root)
	if err != nil {
		return report, err
	}
	report.Artifacts = stats.Artifacts
	return report, nil
}

// bindCloudVault returns the vault root for this session, linking the attached
// vault clone onto $HOME/anvil-vault when the operator has not pointed
// $ANVIL_VAULT somewhere explicit. Linking rather than exporting is deliberate:
// a SessionStart hook cannot put a variable into the environment of the commands
// the agent runs afterwards, but it can put the vault where anvil already looks.
func bindCloudVault() (string, error) {
	if root := os.Getenv("ANVIL_VAULT"); root != "" {
		return root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	target := filepath.Join(home, vaultRemoteMarker)
	if _, err := os.Lstat(target); err == nil {
		return target, nil
	}
	clone, err := discoverVaultClone(home)
	if err != nil {
		return "", err
	}
	if err := os.Symlink(clone, target); err != nil {
		return "", fmt.Errorf("linking vault clone %s: %w", clone, err)
	}
	return target, nil
}

// discoverVaultClone finds the attached vault checkout by reading each candidate
// repo's git remote. Search is breadth-first over the roots a cloud session
// clones into, two levels deep — enough for both a flat <root>/<repo> layout and
// a nested <root>/<owner>/<repo> one.
func discoverVaultClone(home string) (string, error) {
	roots := []string{home, "/workspace", "/home"}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, filepath.Dir(wd))
	}
	for _, root := range roots {
		for _, dir := range childDirs(root) {
			if isVaultClone(dir) {
				return dir, nil
			}
			for _, nested := range childDirs(dir) {
				if isVaultClone(nested) {
					return nested, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no %s clone found: attach the vault repo to this session, or set ANVIL_VAULT", vaultRemoteMarker)
}

func childDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	return dirs
}

func isVaultClone(dir string) bool {
	cfg, err := os.ReadFile(filepath.Join(dir, ".git", "config")) //nolint:gosec // dir is a checkout discovered under the session's own clone roots, not user input
	if err != nil {
		return false
	}
	return strings.Contains(string(cfg), vaultRemoteMarker)
}
