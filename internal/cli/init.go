package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Scaffold an Anvil vault",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var v *core.Vault
			if len(args) == 1 {
				v = &core.Vault{Root: args[0]}
			} else {
				rv, err := core.ResolveVault()
				if err != nil {
					return err
				}
				v = rv
			}
			if err := v.Scaffold(); err != nil {
				return err
			}
			entries, err := schema.EmbeddedFS.ReadDir(".")
			if err != nil {
				return fmt.Errorf("read embedded schemas: %w", err)
			}
			for _, e := range entries {
				b, err := schema.EmbeddedFS.ReadFile(e.Name())
				if err != nil {
					return fmt.Errorf("read %s: %w", e.Name(), err)
				}
				target := filepath.Join(v.SchemasDir(), e.Name())
				if err := os.WriteFile(target, b, 0o644); err != nil {
					return fmt.Errorf("write %s: %w", target, err)
				}
			}
			cmd.Println("vault scaffolded at", v.Root)
			return nil
		},
	}
}
