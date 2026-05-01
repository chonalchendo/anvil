package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/state"
)

func newThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Manage the active thread",
	}
	cmd.AddCommand(
		newThreadActivateCmd(),
		newThreadDeactivateCmd(),
		newThreadCurrentCmd(),
	)
	return cmd
}

func newThreadActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <id>",
		Short: "Set the active thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			path := filepath.Join(v.Root, core.TypeThread.Dir(), id+".md")
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("thread %q not found at %s", id, path)
			}
			if err := state.WriteActiveThread(id); err != nil {
				return err
			}
			cmd.Println("activated", id)
			return nil
		},
	}
}

func newThreadDeactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate",
		Short: "Clear the active thread",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := state.ClearActiveThread(); err != nil {
				return err
			}
			cmd.Println("deactivated")
			return nil
		},
	}
}

func newThreadCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the active thread ID",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, err := state.ReadActiveThread()
			if err != nil {
				return err
			}
			if id == "" {
				cmd.Println("(none)")
				return nil
			}
			cmd.Println(id)
			return nil
		},
	}
}
