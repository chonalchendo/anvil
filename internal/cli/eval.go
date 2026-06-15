package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/adapters/claude"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/eval"
	"github.com/chonalchendo/anvil/internal/index"
)

func newEvalCmd() *cobra.Command {
	var (
		flagJSON    bool
		flagHistory bool
		flagModel   string
		flagTimeout time.Duration
		flagCase    int
	)

	cmd := &cobra.Command{
		Use:   "eval <skill>",
		Short: "Run a skill's evals.json cases through the agent adapter and record the results",
		Long: "Run each eval in <skill>/evals/evals.json through the agent CLI in a fresh\n" +
			"fixture directory, grade the outcome with an LLM judge, and record\n" +
			"{skill, eval_id, pass, cost, duration, model, date} to the vault index.\n" +
			"With --history, print past runs instead of executing.",
		Args: cobra.ExactArgs(1),
		Example: `  anvil eval extracting-skill-from-session
  anvil eval extracting-skill-from-session --case 1 --json
  anvil eval --history extracting-skill-from-session --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			skill := args[0]
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			if flagHistory {
				return runEvalHistory(cmd, db, skill, flagJSON)
			}
			return runEvalSuite(cmd, db, skill, flagJSON, flagCase, flagModel, flagTimeout)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON instead of a table")
	cmd.Flags().BoolVar(&flagHistory, "history", false, "print recorded runs for the skill instead of executing")
	cmd.Flags().StringVar(&flagModel, "model", "claude-sonnet-4-6", "Claude model id for both the skill run and the judge")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 10*time.Minute, "per-spawn timeout")
	cmd.Flags().IntVar(&flagCase, "case", -1, "run only the eval with this id (default: all)")
	return cmd
}

func runEvalSuite(cmd *cobra.Command, db *index.DB, skill string, asJSON bool, caseID int, model string, timeout time.Duration) error {
	// eval routes through the Claude adapter only (the sole adapter anvil ships);
	// "claude-" is build.Router's dispatch key. Reject other ids loudly rather
	// than spawning Claude with a model it does not recognise.
	if !strings.HasPrefix(model, "claude-") {
		return fmt.Errorf("anvil eval runs the Claude adapter only; --model must be a claude-* id, got %q", model)
	}
	suite, err := eval.LoadSuite(skills.FS, skill)
	if err != nil {
		return err
	}
	runner := &eval.Runner{Adapter: claude.New(""), Model: model, Timeout: timeout}

	var results []eval.Result
	for _, c := range suite.Evals {
		if caseID >= 0 && c.ID != caseID {
			continue
		}
		if !asJSON {
			cmd.Printf("running %s/%d (%s)…\n", skill, c.ID, c.Name)
		}
		res, err := runner.RunCase(cmd.Context(), skill, c)
		if err != nil {
			return fmt.Errorf("eval %s/%d: %w", skill, c.ID, err)
		}
		if err := db.RecordEvalRun(index.EvalRunRow{
			Skill: res.Skill, EvalID: res.EvalID, Name: res.Name, Pass: res.Pass,
			Cost: res.Cost, Duration: res.Duration, Model: res.Model, Date: res.Date,
		}); err != nil {
			return err
		}
		results = append(results, res)
	}
	if caseID >= 0 && len(results) == 0 {
		return fmt.Errorf("skill %q has no eval with id %d", skill, caseID)
	}

	if asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Skill string        `json:"skill"`
			Runs  []eval.Result `json:"runs"`
		}{Skill: skill, Runs: results})
	}
	for _, r := range results {
		cmd.Printf("%s  %s/%d %-28s $%.4f  %s\n", passLabel(r.Pass), skill, r.EvalID, r.Name, r.Cost, r.Reason)
	}
	return nil
}

func runEvalHistory(cmd *cobra.Command, db *index.DB, skill string, asJSON bool) error {
	rows, err := db.EvalHistory(skill)
	if err != nil {
		return err
	}
	if asJSON {
		if rows == nil {
			rows = []index.EvalRunRow{}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
	}
	if len(rows) == 0 {
		cmd.Printf("no recorded eval runs for %s\n", skill)
		return nil
	}
	for _, r := range rows {
		cmd.Printf("%s  %s  %s/%d %-28s $%.4f  %s\n", r.Date, passLabel(r.Pass), r.Skill, r.EvalID, r.Name, r.Cost, r.Model)
	}
	return nil
}

func passLabel(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}
