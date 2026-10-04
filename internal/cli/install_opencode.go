package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
