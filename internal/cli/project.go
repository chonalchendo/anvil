package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage project bindings",
	}
	cmd.AddCommand(
		newProjectCurrentCmd(),
		newProjectListCmd(),
		newProjectAdoptCmd(),
		newProjectSwitchCmd(),
	)
	return cmd
}

func newProjectCurrentCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the current project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := core.ResolveProject()
			if err != nil {
				return fmt.Errorf("no current project: %w", err)
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"slug": p.Slug,
					"root": p.Root,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", p.Slug, p.Root)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List adopted projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			projects, err := core.ListProjects()
			if err != nil {
				return err
			}
			for _, p := range projects {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", p.Slug, p.Root)
			}
			return nil
		},
	}
}

func newProjectAdoptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "adopt <slug>",
		Short: "Adopt the current git tree under slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return core.AdoptProject(args[0])
		},
	}
}

func newProjectSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <slug>",
		Short: "Switch the global current-project pointer to slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return core.SwitchProject(args[0])
		},
	}
}
