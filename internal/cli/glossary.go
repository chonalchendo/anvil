package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
)

func newGlossaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "glossary",
		Short:        "Manage vault glossary definitions",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newGlossaryAddCmd())
	return cmd
}

func newGlossaryAddCmd() *cobra.Command {
	var (
		flagDesc   string
		flagUpdate bool
	)
	cmd := &cobra.Command{
		Use:   "add <term> --desc \"...\"",
		Short: "Add a definition to the vault glossary (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			term := args[0]
			if flagDesc == "" {
				return fmt.Errorf("--desc is required")
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			path := glossary.Path(v.Root)
			g, err := glossary.Load(path)
			if err != nil {
				return err
			}
			existing, hadIt := g.Definition(term)
			if hadIt && existing == flagDesc {
				fmt.Fprintln(cmd.OutOrStdout(), path)
				return nil
			}
			if hadIt && !flagUpdate {
				return fmt.Errorf("term %q already defined: %q\n  use --update to overwrite: anvil glossary add %s --desc %q --update",
					term, existing, term, flagDesc)
			}
			if hadIt && flagUpdate {
				g.UpdateDefinition(term, flagDesc)
			} else {
				if err := g.AddDefinition(term, flagDesc); err != nil {
					return err
				}
			}
			if err := g.Save(path); err != nil {
				return fmt.Errorf("saving glossary: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDesc, "desc", "", "one-line definition (required)")
	cmd.Flags().BoolVar(&flagUpdate, "update", false, "rewrite existing term's definition")
	return cmd
}
