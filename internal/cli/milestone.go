package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// newMilestoneCmd groups milestone queries. `status` is the deterministic
// done-signal (anvil.0102).
func newMilestoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Query milestones",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newMilestoneStatusCmd())
	return cmd
}

// newMilestoneStatusCmd reports a milestone's issue counts and per-issue cost,
// and runs each `acceptance:` predicate on the current checkout, reporting met
// or not met.
func newMilestoneStatusCmd() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "status <milestone-id>",
		Short: "Report a milestone's issue counts, per-issue cost and acceptance predicates",
		Args:  cobra.ExactArgs(1),
		Example: `  anvil milestone status anvil.<slug>
  anvil milestone status anvil.<slug> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := indexForRead(v)
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			st, err := db.MilestoneStatus(args[0])
			if err != nil {
				if errors.Is(err, index.ErrArtifactNotInIndex) {
					return fmt.Errorf("%w: %s", ErrArtifactNotFound, args[0])
				}
				return err
			}

			_, path, err := core.ResolveArtifact(v, core.TypeMilestone, args[0])
			if err != nil {
				return fmt.Errorf("%w: %s", ErrArtifactNotFound, args[0])
			}
			m, err := core.LoadArtifact(path)
			if err != nil {
				return err
			}
			fl := checkFinishLine(projectFromArtifact(m, args[0]))
			warnBaseUnchecked(cmd, fl)
			if fl.off() {
				cmd.PrintErrln("warning: HEAD " + fl.Head + " is not the default branch tip " + fl.Base + "; `transition milestone done` refuses here")
			}
			acceptance := runAcceptance(cmd, m, fl.Dir)
			rows, total, scanErr := milestoneCostRows(v, strings.TrimPrefix(args[0], "milestone."))
			if scanErr != nil {
				cmd.PrintErrln("warning: issue scan failed: " + scanErr.Error())
			}
			open := 0
			for _, r := range rows {
				if r.unfinished() {
					open++
				}
			}
			fixed, err := fixesTargets(db, rows)
			if err != nil {
				return err
			}
			outcome := sumOutcome(rows, m.FrontMatter, fixed)
			done := scanErr == nil && open == 0 && len(unmetCriteria(acceptance)) == 0

			if flagJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					index.MilestoneStatus
					Done       bool                `json:"done"`
					Acceptance []acceptanceResult  `json:"acceptance"`
					Issues     []milestoneIssueRow `json:"issues"`
					CostTotal  milestoneCostTotal  `json:"cost_total"`
					Outcome    milestoneOutcome    `json:"outcome"`
				}{st, done, acceptance, rows, total, outcome})
			}
			out := cmd.OutOrStdout() // data, not diagnostics: must survive 2>/dev/null
			fmt.Fprintf(out, "%s\t%d/%d resolved\tdone=%t\n", st.Milestone, st.Resolved, st.Total, done)
			for i, r := range acceptance {
				verdict := "met"
				if !r.Met {
					verdict = "not met"
				}
				fmt.Fprintf(out, "AC %d\t%s\t%s\t%s\n", i+1, verdict, r.detail(), tableCell(r.Criterion))
			}
			for _, r := range rows {
				fmt.Fprintln(out, r.line())
			}
			fmt.Fprintln(out, outcome.line())
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit status, acceptance, cost rows and outcome as JSON")
	return cmd
}
