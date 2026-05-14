// Package cli holds the cobra command tree.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// flagLeadingErrRE matches an error message whose first token is a flag-shaped
// identifier (one or more leading dashes followed by a letter). Used to bypass
// fang's first-word title-case transform, which mangles `--body` → `--Body` and
// breaks copy-paste of the suggested fix.
var flagLeadingErrRE = regexp.MustCompile(`^-+[A-Za-z]`)

// Execute is the CLI entrypoint, invoked by cmd/anvil/main.go.
func Execute(ctx context.Context) error {
	return fang.Execute(ctx, newRootCmd(), fang.WithErrorHandler(errorHandler), fang.WithVersion(resolveVersion()))
}

// errorHandler preserves whitespace for multi-line errors (e.g. the structured
// drift block from `create`); fang's default renders single-line messages
// inside a lipgloss box that reflows embedded newlines into one paragraph.
// When the message leads with a flag name, the first-word title-case transform
// is suppressed so `--body` / `--description` survive verbatim for copy-paste.
func errorHandler(w io.Writer, styles fang.Styles, err error) {
	msg := err.Error()
	if strings.Contains(msg, "\n") {
		_, _ = fmt.Fprintln(w, msg)
		return
	}
	if flagLeadingErrRE.MatchString(msg) {
		styles.ErrorText = styles.ErrorText.UnsetTransform()
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
			refreshSkillsIfStale(cmd)
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
		newReindexCmd(),
		newTransitionCmd(),
		newRenameCmd(),
	)
	return cmd
}
