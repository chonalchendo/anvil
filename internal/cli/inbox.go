package cli

import (
	"github.com/spf13/cobra"
)

func newInboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Manage inbox artifacts",
	}
	cmd.AddCommand(newInboxPromoteCmd())
	return cmd
}

func newInboxPromoteCmd() *cobra.Command {
	c := newPromoteCmd()
	c.Use = "promote <id>"
	return c
}
