// Package core implements vault, project, and artifact primitives shared
// by every anvil CLI verb. No cobra imports here — this package must be
// usable from tests without a command tree.
package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// VaultDirs lists every directory Scaffold creates under the vault root.
var VaultDirs = []string{
	"00-inbox",
	"05-projects",
	"10-sessions/raw",
	"10-sessions/distilled",
	"20-learnings",
	"30-decisions",
	"40-skills",
	"50-sweeps",
	"60-threads",
	"70-issues",
	"80-plans",
	"85-milestones",
	"90-bases",
	"99-archive",
	"_meta",
	"schemas",
}

// Vault is the on-disk vault tree under ~/anvil-vault/ (or $ANVIL_VAULT).
type Vault struct {
	Root string
}

// ResolveVault returns the vault implied by $ANVIL_VAULT or the default
// ~/anvil-vault/. The directory is not required to exist.
func ResolveVault() (*Vault, error) {
	if v := os.Getenv("ANVIL_VAULT"); v != "" {
		return &Vault{Root: v}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home: %w", err)
	}
	return &Vault{Root: filepath.Join(home, "anvil-vault")}, nil
}

// Scaffold creates every directory in VaultDirs under v.Root. It is idempotent:
// existing dirs and user content are never touched.
func (v *Vault) Scaffold() error {
	if v.Root == "" {
		return errors.New("vault root unset")
	}
	for _, d := range VaultDirs {
		if err := os.MkdirAll(filepath.Join(v.Root, d), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

// SchemasDir returns the canonical path where schemas live in the vault.
func (v *Vault) SchemasDir() string { return filepath.Join(v.Root, "schemas") }

// BasesDir returns the canonical path where Obsidian Bases dashboards live.
func (v *Vault) BasesDir() string { return filepath.Join(v.Root, "90-bases") }

// EnableObsidianCorePlugin turns on the named Obsidian core plugin in
// .obsidian/core-plugins.json, creating the file (and .obsidian/) when absent.
// It is non-destructive: any other plugin states already in the file are
// preserved, so a user's existing Obsidian config survives init.
func (v *Vault) EnableObsidianCorePlugin(name string) error {
	dir := filepath.Join(v.Root, ".obsidian")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return fmt.Errorf("mkdir .obsidian: %w", err)
	}
	path := filepath.Join(dir, "core-plugins.json")
	plugins := map[string]bool{}
	switch b, err := os.ReadFile(path); { //nolint:gosec // path is vault-internal .obsidian config, not user input
	case err == nil:
		if err := json.Unmarshal(b, &plugins); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("reading %s: %w", path, err)
	}
	plugins[name] = true
	out, err := json.MarshalIndent(plugins, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling core-plugins: %w", err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
