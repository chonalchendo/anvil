package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// Read-only subcommands over the runtime-inserted build telemetry tables. They
// open the index directly rather than via indexForRead — a stale .md elsewhere
// must not gate a query over runtime-only telemetry (matches internal/cli/eval.go).

// newBuildRunsCmd lists build runs most-recent-first, so a completed run's
// run_id is retrievable for `anvil build tasks <run-id>`. Read-only over the
// index's build_runs table, filterable by --project/--milestone.
func newBuildRunsCmd() *cobra.Command {
	var flagJSON bool
	var flagProject, flagMilestone string
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List build runs most-recent-first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			rows, err := db.ListBuildRuns(flagProject, flagMilestone)
			if err != nil {
				return err
			}
			if flagJSON {
				return output.WriteListJSON(cmd.OutOrStdout(), rows, len(rows), len(rows))
			}
			if len(rows) == 0 {
				cmd.PrintErrln("no build runs")
				return nil
			}
			for _, r := range rows {
				dry := ""
				if r.DryRun {
					dry = "\tdry-run"
				}
				cmd.Printf("%s\t%s\t%s\t%d tasks%s\n", r.RunID, r.StartedAt, r.Milestone, r.Tasks, dry)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit build runs as the canonical list envelope")
	cmd.Flags().StringVar(&flagProject, "project", "", "restrict to runs in this project (exact match)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "restrict to runs under this milestone slug")
	return cmd
}

// newBuildTasksCmd queries the per-task telemetry a build run persisted, keyed
// by run id. Read-only over the index's build_tasks table.
func newBuildTasksCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "tasks <run-id>",
		Short: "Show per-task telemetry for a build run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			rows, err := db.BuildTasksByRun(args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				return output.WriteListJSON(cmd.OutOrStdout(), rows, len(rows), len(rows))
			}
			if len(rows) == 0 {
				cmd.PrintErrf("no telemetry for run %s\n", args[0])
				return nil
			}
			for _, r := range rows {
				cmd.Printf("%s\t%s\t%s\t%s\t%d→%d tok\t$%.4f\texit %d\n",
					r.TaskID, r.Phase, r.Model, r.Outcome, r.TokensIn, r.TokensOut, r.CostUSD, r.VerifyExit)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit per-task telemetry as the canonical list envelope")
	return cmd
}
