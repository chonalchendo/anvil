// Package cli holds the cobra command tree.
package cli

import (
	"context"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Execute is the CLI entrypoint, invoked by cmd/anvil/main.go.
func Execute(ctx context.Context) error {
	return fang.Execute(ctx, newRootCmd())
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "anvil",
		Short:         "Anvil — agentic-development methodology",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newWhereCmd(),
		newInitCmd(),
		newInboxCmd(),
		newCreateCmd(),
		newShowCmd(),
		newListCmd(),
		newLinkCmd(),
		newSetCmd(),
		newProjectCmd(),
		newValidateCmd(),
		newMigrateCmd(),
		newThreadCmd(),
		newSessionCmd(),
		newInstallCmd(),
		newGlossaryCmd(),
		newTagsCmd(),
	)
	return cmd
}
