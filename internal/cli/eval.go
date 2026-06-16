package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Record and query externally-produced skill-eval results",
		Long: `Record and query skill-eval results produced by an external harness.

anvil ingests the record, it does not run or grade evals: a harness such as
Anthropic's skill-creator owns execution, transcript capture, and grading,
and emits grading.json / history.json (see docs/eval-methodology.md). anvil
reads those into vault.db so skill-confidence is queryable over time.`,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newEvalIngestCmd(), newEvalHistoryCmd())
	return cmd
}

func newEvalIngestCmd() *cobra.Command {
	var (
		flagFrom   string
		flagSource string
		flagRef    string
		flagJSON   bool
	)
	cmd := &cobra.Command{
		Use:   "ingest <skill>",
		Short: "Record a skill-creator grading.json or history.json into vault.db",
		Long: `Read a skill-creator grading.json (one graded run) or history.json
(per-version progression) and append the result(s) to the eval_runs table.

A grading.json records one run from summary.{passed,failed,total,pass_rate};
--ref labels it (e.g. the eval version). A history.json records one run per
iteration, keyed by its version, carrying expectation_pass_rate (counts are
null — the iteration schema omits them).

--json emits the recorded run as an object, or the recorded runs as a
{items,...} list envelope when more than one.`,
		Example: `  anvil eval ingest writing-product-design --from run/grading.json
  anvil eval ingest pdf --from improve/history.json --source skill-creator --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skill := args[0]
			if flagFrom == "" {
				return fmt.Errorf("--from <path> is required")
			}
			data, err := os.ReadFile(flagFrom) //nolint:gosec // path is an explicit user-supplied argument
			if err != nil {
				return fmt.Errorf("reading %s: %w", flagFrom, err)
			}
			rows, err := parseEvalFile(data, skill, flagRef)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", flagFrom, err)
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			date := time.Now().UTC().Format("2006-01-02")
			for i := range rows {
				rows[i].Source = flagSource
				rows[i].Date = date
				if err := db.InsertEvalRun(rows[i]); err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			if flagJSON {
				if len(rows) == 1 {
					b, err := json.Marshal(rows[0])
					if err != nil {
						return err
					}
					fmt.Fprintln(out, string(b))
					return nil
				}
				return output.WriteListJSON(out, rows, len(rows), len(rows))
			}
			for _, r := range rows {
				fmt.Fprintf(out, "recorded eval: %s ref=%q pass_rate=%.2f (%s)\n", r.Skill, r.Ref, r.PassRate, r.Source)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "path to a skill-creator grading.json or history.json (required)")
	cmd.Flags().StringVar(&flagSource, "source", "skill-creator", "harness that produced the result")
	cmd.Flags().StringVar(&flagRef, "ref", "", "eval/version identifier for a grading.json (e.g. v2)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON")
	return cmd
}

func newEvalHistoryCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:     "history <skill>",
		Short:   "Show a skill's recorded eval runs over time",
		Example: `  anvil eval history writing-product-design --json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skill := args[0]
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			runs, err := db.EvalHistory(skill)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if flagJSON {
				return output.WriteListJSON(out, runs, len(runs), len(runs))
			}
			for _, r := range runs {
				ref := r.Ref
				if ref == "" {
					ref = "-"
				}
				fmt.Fprintf(out, "%s\t%s\t%.2f\t%s\n", r.Date, ref, r.PassRate, r.Source)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON")
	return cmd
}

// gradingFile and historyFile capture the subset of skill-creator's committed
// schema (references/schemas.md) that anvil consumes.
type gradingFile struct {
	Summary *struct {
		Passed   int     `json:"passed"`
		Failed   int     `json:"failed"`
		Total    int     `json:"total"`
		PassRate float64 `json:"pass_rate"`
	} `json:"summary"`
}

type historyFile struct {
	Iterations []struct {
		Version             string  `json:"version"`
		ExpectationPassRate float64 `json:"expectation_pass_rate"`
	} `json:"iterations"`
}

// parseEvalFile distinguishes a grading.json (has summary) from a history.json
// (has iterations) by content and returns the run(s) to record. Source and
// date are stamped by the caller.
func parseEvalFile(data []byte, skill, ref string) ([]index.EvalRun, error) {
	var g gradingFile
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if g.Summary != nil {
		s := g.Summary
		return []index.EvalRun{{
			Skill: skill, Ref: ref,
			Passed: &s.Passed, Failed: &s.Failed, Total: &s.Total,
			PassRate: s.PassRate,
		}}, nil
	}

	var h historyFile
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(h.Iterations) > 0 {
		rows := make([]index.EvalRun, len(h.Iterations))
		for i, it := range h.Iterations {
			rows[i] = index.EvalRun{Skill: skill, Ref: it.Version, PassRate: it.ExpectationPassRate}
		}
		return rows, nil
	}

	return nil, fmt.Errorf("no summary (grading.json) or iterations (history.json) found")
}
