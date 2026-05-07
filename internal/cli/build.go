package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newBuildCmd() *cobra.Command {
	var (
		flagConcurrency int
		flagCwd         string
		flagJSON        bool
		flagDryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "build <plan-id>",
		Short: "Walk a plan's wave graph and dispatch each task to its agent CLI",
		Args:  cobra.ExactArgs(1),
		Example: `  anvil build anvil.refactor-auth --dry-run
  anvil build anvil.refactor-auth --concurrency 2 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			path := filepath.Join(v.Root, core.TypePlan.Dir(), args[0]+".md")

			a, err := core.LoadArtifact(path)
			if err != nil {
				return ErrArtifactNotFound
			}
			if err := schema.Validate("plan", a.FrontMatter); err != nil {
				cmd.PrintErrln(err)
				return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
			}
			p, err := core.LoadPlan(path)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
			}
			if err := core.ValidatePlan(p); err != nil {
				cmd.PrintErrln(err)
				return err
			}

			cwd := flagCwd
			if cwd == "" {
				cwd, _ = filepath.Abs(".")
			}

			opts := build.Options{
				Concurrency: flagConcurrency,
				Cwd:         cwd,
				DryRun:      flagDryRun,
				JSON:        flagJSON,
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
				Router:      build.Router{}, // sub-projects 2 / 4 register adapters here
			}
			_, err = build.Build(cmd.Context(), p, opts)
			return err
		},
	}

	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "max in-flight tasks per wave")
	cmd.Flags().StringVar(&flagCwd, "cwd", "", "agent working directory (default: current directory)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit one JSON record per task to stdout")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "print waves + dispatched tasks; do not call any adapter")
	return cmd
}
