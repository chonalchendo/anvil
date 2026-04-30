// Package core implements vault, project, and artifact primitives shared
// by every anvil CLI verb. No cobra imports here — this package must be
// usable from tests without a command tree.
package core

import (
	"errors"
	"fmt"
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
	"90-moc/dashboards",
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
		if err := os.MkdirAll(filepath.Join(v.Root, d), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

// SchemasDir returns the canonical path where schemas live in the vault.
func (v *Vault) SchemasDir() string { return filepath.Join(v.Root, "schemas") }
