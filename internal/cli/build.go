package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/adapters/claude"
	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// newBuildCmd builds the issue-graph dispatch loop. The command is the driver
// in contract.anvil.build-orchestration-contract: it selects work — the ready-issue
// frontier from the index — and hands the engine pre-built task waves. It does
// not own dispatch mechanics. Hidden pending the live-spawn (AC #3) and
// telemetry (AC #4) milestone slices; --dry-run is the supported path today.
func newBuildCmd() *cobra.Command {
	var (
		flagConcurrency int
		flagCwd         string
		flagJSON        bool
		flagDryRun      bool
		flagProject     string
		flagMilestone   string
	)

	cmd := &cobra.Command{
		Use:    "build",
		Short:  "Dispatch the ready-issue graph, one agent per ready issue",
		Hidden: true,
		Args:   cobra.NoArgs,
		Example: `  anvil build --dry-run
  anvil build --milestone anvil.<slug> --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := indexForRead(v)
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			rows, err := db.ListReady(string(core.TypeIssue), index.QueryFilters{Project: flagProject})
			if err != nil {
				return err
			}
			tasks := readyUnitsToTasks(selectReadyUnits(rows, flagMilestone))
			if len(tasks) == 0 {
				cmd.PrintErrln("no ready issues to dispatch")
				return nil
			}

			cwd := flagCwd
			if cwd == "" {
				cwd, err = filepath.Abs(".")
				if err != nil {
					return fmt.Errorf("resolving cwd: %w", err)
				}
			}

			// Text dry-run: the engine emits per-task records only in --json
			// mode, so the driver lists the selected frontier here to honour
			// the flag's promise. --json dry-run is left to the engine's records.
			if flagDryRun && !flagJSON {
				for _, t := range tasks {
					cmd.Println(t.ID)
				}
			}

			opts := build.Options{
				Concurrency: flagConcurrency,
				Cwd:         cwd,
				DryRun:      flagDryRun,
				JSON:        flagJSON,
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
				Router: build.Router{
					"claude-": claude.New(""),
				},
			}
			// The ready frontier is one wave: ready issues have no unresolved
			// depends_on, so they are mutually independent. The dependency graph
			// advances across invocations as the human merges each PR and the
			// next frontier unblocks — a single run must not dispatch a later
			// wave while earlier PRs sit unmerged.
			_, err = build.Build(cmd.Context(), [][]core.Task{tasks}, opts)
			return err
		},
	}

	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "max in-flight tasks")
	cmd.Flags().StringVar(&flagCwd, "cwd", "", "agent working directory (default: current directory)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit one JSON record per task to stdout")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "print the dispatched ready issues; do not call any adapter")
	cmd.Flags().StringVar(&flagProject, "project", "", "restrict to ready issues in this project (exact match; default: all)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "restrict to ready issues under this milestone slug")
	return cmd
}

// readyUnitsToTasks maps the priority-ordered ready frontier to dispatch tasks.
// Each unit becomes a completing-issue task whose body carries the assembled
// start-context (goal, severity, milestone, governing contracts, path) — the
// same context `anvil next` hands an interactive agent, so a dispatched agent
// starts from the unit-with-context rather than a bare id. Milestone and
// contracts lines are omitted when empty so the body carries no blank scaffolding.
func readyUnitsToTasks(units []readyUnit) []core.Task {
	tasks := make([]core.Task, 0, len(units))
	for _, u := range units {
		var b strings.Builder
		fmt.Fprintf(&b, "Complete anvil issue %s end-to-end to PR-opened using the completing-issue skill. The human owns the merge.\n\n", u.ID)
		fmt.Fprintf(&b, "Goal: %s\n", u.Goal)
		fmt.Fprintf(&b, "Severity: %s\n", u.Severity)
		if u.Milestone != "" {
			fmt.Fprintf(&b, "Milestone: %s\n", u.Milestone)
		}
		if len(u.Contracts) > 0 {
			fmt.Fprintf(&b, "Governing contracts: %s\n", strings.Join(u.Contracts, ", "))
		}
		fmt.Fprintf(&b, "Issue path: %s\n", u.Path)

		tasks = append(tasks, core.Task{
			ID:           u.ID,
			SkillsToLoad: []string{"completing-issue"},
			Body:         b.String(),
		})
	}
	return tasks
}
