// Package cli holds the cobra command tree.
package cli

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Execute is the CLI entrypoint, invoked by cmd/anvil/main.go.
func Execute(ctx context.Context) error {
	return fang.Execute(ctx, newRootCmd())
}

func newRootCmd() *cobra.Command {
	var flagVault, flagProject string
	cmd := &cobra.Command{
		Use:           "anvil",
		Short:         "Anvil — agentic-development methodology",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if flagVault != "" {
				_ = os.Setenv("ANVIL_VAULT", flagVault)
			}
			if flagProject != "" {
				_ = os.Setenv("ANVIL_PROJECT", flagProject)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&flagVault, "vault", "", "override vault root (precedence: flag > $ANVIL_VAULT > cwd resolution)")
	cmd.PersistentFlags().StringVar(&flagProject, "project", "", "override current project slug (precedence: flag > $ANVIL_PROJECT > cwd resolution)")
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
	)
	return cmd
}
