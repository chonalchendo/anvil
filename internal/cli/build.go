package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/adapters/claude"
	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/cli/output"
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

			// Exit predicate: a milestone-scoped run stops once every linked
			// issue is resolved. The driver consults the deterministic
			// done-signal (anvil.0102) before selecting work, so the loop has a
			// principled stopping point rather than dispatching into an
			// already-complete milestone. A slug absent from the index is not
			// done — fall through to the filter, which yields the no-ready
			// notice (build's --milestone tolerates unmatched slugs).
			if flagMilestone != "" {
				st, err := db.MilestoneStatus(flagMilestone)
				switch {
				case errors.Is(err, index.ErrArtifactNotInIndex):
					// fall through
				case err != nil:
					return err
				case st.Done:
					cmd.PrintErrf("milestone %s is done (%d/%d resolved); nothing to dispatch\n", st.Milestone, st.Resolved, st.Total)
					return nil
				}
			}

			rows, err := db.ListReady(string(core.TypeIssue), index.QueryFilters{Project: flagProject})
			if err != nil {
				return err
			}
			units := selectReadyUnits(rows, flagMilestone)
			tasks := readyUnitsToTasks(units)
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

			// A real run claims + cuts each ready issue's canonical worktree
			// before dispatch and pins it as the task Cwd, so the spawned worker
			// lands its PR on the deterministic branch anvil/<slug> the engine
			// already holds — `fleet status` correlation and the advance-gate
			// (anvil.0112) then operate on a branch the driver knows rather than
			// one it must trust the worker to derive. The driver owns this single
			// claim+cut (build-orchestration-contract: it owns work-selection and
			// the vault writes that reserve it); completing-issue's build path
			// skips its own. Dry-run never touches the vault or git.
			if !flagDryRun {
				if err := claimAndCutForBuild(v, cmd.ErrOrStderr(), units, tasks); err != nil {
					return err
				}
			}

			runID := newRunID()
			startedAt := time.Now().UTC().Format(time.RFC3339)

			// Text dry-run: list the selected frontier.
			if flagDryRun && !flagJSON {
				for _, t := range tasks {
					cmd.Println(t.ID)
				}
			}

			opts := build.Options{
				Concurrency: flagConcurrency,
				Cwd:         cwd,
				DryRun:      flagDryRun,
				// Dry-run JSON is the plan envelope below, not the live NDJSON
				// stream — so the engine stays quiet on stdout in that path.
				JSON:   flagJSON && !flagDryRun,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
				Router: build.Router{
					"claude-": claude.New(""),
				},
			}
			// The ready frontier is one wave: ready issues have no unresolved
			// depends_on, so they are mutually independent. The dependency graph
			// advances across invocations as the human merges each PR and the
			// next frontier unblocks — a single run must not dispatch a later
			// wave while earlier PRs sit unmerged.
			sum, buildErr := build.Build(cmd.Context(), [][]core.Task{tasks}, opts)

			// Persist per-task telemetry keyed by run, queryable via
			// `anvil build tasks <run-id>`. The driver is where the engine's
			// in-memory Summary meets the index (build-orchestration-contract).
			// Build always returns a non-nil Summary (empty on a pre-wave cancel).
			// A real build's exit still wins; a telemetry failure there only warns.
			if terr := recordBuildTelemetry(db, runID, startedAt, flagProject, flagMilestone, flagDryRun, sum); terr != nil {
				if buildErr != nil {
					cmd.PrintErrf("warning: build telemetry not persisted: %v\n", terr)
				} else {
					return fmt.Errorf("persisting build telemetry: %w", terr)
				}
			}

			// JSON dry-run: emit a single plan envelope so consumers can assert
			// per-task fields (config_dir uniqueness, auto_merge) and the run id
			// with a plain jq path rather than slurp-mode. The engine owns the
			// per-task record shape; the driver only hands it the waves.
			if flagDryRun && flagJSON {
				return build.PlanJSON(cmd.OutOrStdout(), runID, [][]core.Task{tasks})
			}
			return buildErr
		},
	}

	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "max in-flight tasks")
	cmd.Flags().StringVar(&flagCwd, "cwd", "", "agent working directory (default: current directory)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit one JSON record per task to stdout")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "print the dispatched ready issues; do not call any adapter")
	cmd.Flags().StringVar(&flagProject, "project", "", "restrict to ready issues in this project (exact match; default: all)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "restrict to ready issues under this milestone slug")
	cmd.AddCommand(newBuildTasksCmd())
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
			// build_tasks is runtime-inserted, not derived from vault markdown, so
			// open directly rather than via indexForRead — a stale .md elsewhere
			// must not gate a query over runtime-only telemetry (matches the
			// eval history read sibling, internal/cli/eval.go).
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
				cmd.Printf("%s\t%s\t%s\t%d→%d tok\t$%.4f\texit %d\n",
					r.TaskID, r.Model, r.Outcome, r.TokensIn, r.TokensOut, r.CostUSD, r.VerifyExit)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit per-task telemetry as the canonical list envelope")
	return cmd
}

// recordBuildTelemetry projects the engine's in-memory Summary onto build_runs /
// build_tasks rows and persists them. Dry-run rows carry the planned model/effort
// with zero token/cost columns so a plan is queryable like a real run.
func recordBuildTelemetry(db *index.DB, runID, startedAt, project, milestone string, dryRun bool, sum *build.Summary) error {
	tasks := make([]index.BuildTask, 0, len(sum.Outcomes))
	for _, oc := range sum.Outcomes {
		tasks = append(tasks, index.BuildTask{
			RunID:       runID,
			TaskID:      oc.TaskID,
			Wave:        oc.Wave,
			Model:       oc.Model,
			Effort:      oc.Effort,
			Outcome:     oc.Outcome,
			TokensIn:    oc.Result.Tokens.Input,
			TokensOut:   oc.Result.Tokens.Output,
			CacheRead:   oc.Result.Tokens.CacheRead,
			CacheWrite:  oc.Result.Tokens.CacheWrite,
			CostUSD:     oc.Result.CostUSD,
			DurationMS:  oc.Duration.Milliseconds(),
			AgentTimeMS: oc.Result.AgentTime.Milliseconds(),
			VerifyExit:  oc.Result.ExitCode,
		})
	}
	if err := db.InsertBuildRun(index.BuildRun{
		RunID:     runID,
		StartedAt: startedAt,
		Project:   project,
		Milestone: milestone,
		DryRun:    dryRun,
		Tasks:     len(tasks),
	}); err != nil {
		return err
	}
	return db.InsertBuildTasks(tasks)
}

// newRunID returns a sortable, collision-resistant build run id: a UTC timestamp
// prefix plus random suffix, so runs order chronologically and never collide.
func newRunID() string {
	var b [6]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error on supported platforms
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
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

// buildClaimOwner is stamped on issues `anvil build` claims, marking the claim
// as engine-initiated so a worker's completing-issue build path (and `anvil
// fleet status`) can tell a build dispatch from an interactive claim.
const buildClaimOwner = "anvil-build"

// claimAndCutForBuild claims each ready issue open→in-progress under
// buildClaimOwner and cuts its canonical worktree (<project>/<slug>), pinning that
// worktree as the matching task's Cwd. units[i] and tasks[i] are the same
// frontier in the same order. The driver — not the spawned worker — owns this
// single claim+cut, so every worker lands its PR on the deterministic branch
// the engine already holds (build-orchestration-contract). The cut precedes the
// status write (mirroring the interactive transition) so a cut failure leaves
// the issue open with no half-applied claim; any error aborts the run before an
// agent spawns rather than dispatching a worker with no worktree.
func claimAndCutForBuild(v *core.Vault, errW io.Writer, units []readyUnit, tasks []core.Task) error {
	stamp := time.Now().UTC().Format("2006-01-02")
	for i := range units {
		a, err := core.LoadArtifact(units[i].Path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", units[i].ID, err)
		}
		wt, err := doCutWorktree(errW, a, units[i].ID, "", "")
		if err != nil {
			return err
		}
		a.FrontMatter["status"] = "in-progress"
		a.FrontMatter["owner"] = buildClaimOwner
		a.FrontMatter["updated"] = stamp
		if err := a.Save(); err != nil {
			return fmt.Errorf("claiming %s: %w", units[i].ID, err)
		}
		if err := indexAfterSave(v, a); err != nil {
			return fmt.Errorf("indexing claim of %s: %w", units[i].ID, err)
		}
		tasks[i].Cwd = wt
	}
	return nil
}
