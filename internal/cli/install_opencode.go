package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/agents"
	"github.com/chonalchendo/anvil/internal/installer"
)

// resolveOpenCodeConfigDir returns OpenCode's config dir:
// $OPENCODE_CONFIG_DIR if set, else ~/.config/opencode; agents land under
// its agents/ subdir.
func resolveOpenCodeConfigDir() (string, error) {
	if d := os.Getenv("OPENCODE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "opencode"), nil
}

func runInstallOpenCodeAgents(cmd *cobra.Command, dir string, uninstall, force bool) error {
	if uninstall {
		changed, err := installer.RemoveOpenCodeAgents(agents.FS, dir)
		if err != nil {
			return fmt.Errorf("removing opencode agents: %w", err)
		}
		if changed {
			cmd.Println("removed anvil agents from", dir)
		} else {
			cmd.Println("no anvil agents found at", dir)
		}
		return nil
	}
	changed, err := installer.InstallOpenCodeAgents(agents.FS, dir, force)
	if err != nil {
		return fmt.Errorf("installing opencode agents: %w", err)
	}
	if changed {
		cmd.Println("installed anvil agents (embedded bundle) as OpenCode subagent markdown into", dir)
	} else {
		cmd.Println("anvil agents up to date at", dir)
	}
	return nil
}
