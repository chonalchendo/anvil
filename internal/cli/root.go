// Package cli holds the cobra command tree.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Execute is the CLI entrypoint, invoked by cmd/anvil/main.go.
func Execute(ctx context.Context) error {
	return fang.Execute(ctx, newRootCmd(), fang.WithErrorHandler(errorHandler))
}

// errorHandler preserves whitespace for multi-line errors (e.g. the structured
// drift block from `create`); fang's default renders single-line messages
// inside a lipgloss box that reflows embedded newlines into one paragraph.
func errorHandler(w io.Writer, styles fang.Styles, err error) {
	if strings.Contains(err.Error(), "\n") {
		_, _ = fmt.Fprintln(w, err.Error())
		return
	}
	fang.DefaultErrorHandler(w, styles, err)
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
		newCreateCmd(),
		newBuildCmd(),
		newShowCmd(),
		newListCmd(),
		newLinkCmd(),
		newSetCmd(),
		newProjectCmd(),
		newValidateCmd(),
		newMigrateCmd(),
		newThreadCmd(),
		newPromoteCmd(),
		newInstallCmd(),
		newTagsCmd(),
	)
	return cmd
}
