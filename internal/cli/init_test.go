package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestInit_CreatesAllVaultDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANVIL_VAULT", dir)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, d := range core.VaultDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("missing %s: %v", d, err)
		}
	}
}

func TestInit_PathArg(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "custom")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"init", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "00-inbox")); err != nil {
		t.Errorf("expected vault at %s", dir)
	}
}
