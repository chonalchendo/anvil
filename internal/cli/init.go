package cli

import (
	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
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
			cmd.Println("vault scaffolded at", v.Root)
			return nil
		},
	}
}
