package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/adapters/claude"
	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// newBuildCmd builds the issue-graph dispatch loop. The command is the driver
// in contract.anvil.build-orchestration: it selects work — the ready-issue
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
			tasks := readyIssuesToTasks(rows, flagMilestone)
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

// readyIssuesToTasks maps ready-issue index rows to dispatch tasks: each ready
// issue becomes a task that runs completing-issue to PR-opened. When milestone
// is set, rows are filtered by their milestone frontmatter — which the index
// does not store, so the artifact is loaded to read it.
func readyIssuesToTasks(rows []index.ArtifactRow, milestone string) []core.Task {
	tasks := make([]core.Task, 0, len(rows))
	for _, r := range rows {
		if milestone != "" {
			a, err := core.LoadArtifact(r.Path)
			if err != nil || milestoneSlug(a.FrontMatter["milestone"]) != milestone {
				continue
			}
		}
		tasks = append(tasks, core.Task{
			ID:           r.ID,
			SkillsToLoad: []string{"completing-issue"},
			Body: fmt.Sprintf(
				"Complete anvil issue %s end-to-end to PR-opened using the completing-issue skill. The human owns the merge.",
				r.ID,
			),
		})
	}
	return tasks
}
