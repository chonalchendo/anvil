package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/adapters/claude"
	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
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
		Use:    "build <plan-id>",
		Short:  "Walk a plan's wave graph and dispatch each task to its agent CLI",
		Hidden: true, // deferred pending Phase B revival; see decision.consolidate-anvil-surface.0003
		Args:   cobra.ExactArgs(1),
		Example: `  anvil build anvil.refactor-auth --dry-run
  anvil build anvil.refactor-auth --concurrency 2 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// anvil build is deferred pending revival of milestone
			// exit-phase-b-dogfood-the-build-orchestrator. The plan type it
			// depends on is deprecated; Phase B economics are in flux. Running
			// anyway, but callers should expect this surface to change.
			cmd.PrintErrln("notice: anvil build is deferred pending Phase B revival" +
				" (milestone: exit-phase-b-dogfood-the-build-orchestrator;" +
				" decision: consolidate-anvil-surface.0003)")
			planID := args[0]
			// Reject path-traversal segments before composing a filesystem path
			// — args[0] is user input and could otherwise escape the plan dir.
			if strings.ContainsAny(planID, `/\`) || strings.Contains(planID, "..") {
				return fmt.Errorf("invalid plan-id %q", planID)
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			path := filepath.Join(v.Root, core.TypePlan.Dir(), planID+".md")

			a, err := core.LoadArtifact(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return ErrArtifactNotFound
				}
				return fmt.Errorf("loading plan %q: %w", planID, err)
			}
			if err := schema.Validate("plan", a.FrontMatter); err != nil {
				cmd.PrintErrln(err)
				return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
			}
			p, err := core.LoadPlan(path)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
			}
			if err := core.ValidatePlan(p); err != nil {
				cmd.PrintErrln(err)
				return err
			}

			cwd := flagCwd
			if cwd == "" {
				cwd, err = filepath.Abs(".")
				if err != nil {
					return fmt.Errorf("resolving cwd: %w", err)
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
			sum, err := build.Build(cmd.Context(), p, opts)
			if sum != nil {
				persistErr := persistTraces(v.Root, p, sum)
				if persistErr != nil {
					cmd.PrintErrln("warning: failed to persist traces:", persistErr)
				}
			}
			return err
		},
	}

	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "max in-flight tasks per wave")
	cmd.Flags().StringVar(&flagCwd, "cwd", "", "agent working directory (default: current directory)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit one JSON record per task to stdout")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "print waves + dispatched tasks; do not call any adapter")
	return cmd
}

// persistTraces writes task outcomes from a completed build run into the vault
// index so they are queryable by `anvil export traces`. Dry-run tasks are
// omitted (they carry no real outcome). A DB error is non-fatal — build
// succeeded; the caller logs it as a warning.
func persistTraces(vaultRoot string, p *core.Plan, sum *build.Summary) error {
	db, err := index.Open(index.DBPath(vaultRoot))
	if err != nil {
		return fmt.Errorf("opening index: %w", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	now := index.NowUTC()
	for _, task := range p.Tasks {
		oc, ok := sum.Outcomes[task.ID]
		if !ok || oc.Outcome == "skipped_dry_run" {
			continue
		}
		tr := index.Trace{
			TaskID:     oc.TaskID,
			Prompt:     oc.Prompt,
			Outcome:    oc.Outcome,
			Model:      oc.Model,
			Effort:     oc.Effort,
			DurationMS: oc.Duration.Milliseconds(),
			CostUSD:    oc.Result.CostUSD,
			RecordedAt: now,
		}
		if err := db.InsertTrace(tr); err != nil {
			return err
		}
	}
	return nil
}
