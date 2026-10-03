package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/agents"
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

// runInstallTranslatedAgents is the shared install/uninstall wrapper for the
// markdown-emitting targets (pi, ante, opencode).
func runInstallTranslatedAgents(
	cmd *cobra.Command, dir string, uninstall, force bool, target, shape string,
	install func(fs.FS, string, bool) (bool, error),
	remove func(fs.FS, string) (bool, error),
) error {
	if uninstall {
		changed, err := remove(agents.FS, dir)
		if err != nil {
			return fmt.Errorf("removing %s agents: %w", target, err)
		}
		if changed {
			cmd.Println("removed anvil agents from", dir)
		} else {
			cmd.Println("no anvil agents found at", dir)
		}
		return nil
	}
	changed, err := install(agents.FS, dir, force)
	if err != nil {
		return fmt.Errorf("installing %s agents: %w", target, err)
	}
	if changed {
		cmd.Println("installed anvil agents (embedded bundle) as", shape, "into", dir)
	} else {
		cmd.Println("anvil agents up to date at", dir)
	}
	return nil
}
