package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// newMilestoneCmd groups milestone queries. `status` is the deterministic
// done-signal the build loop consults as its exit predicate (anvil.0102).
func newMilestoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Query milestones",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newMilestoneStatusCmd())
	return cmd
}

// newMilestoneStatusCmd reports whether a milestone is done — every issue linked
// to it is resolved — as resolved-vs-total counts and a boolean verdict. This is
// the machine-verifiable exit criterion `anvil build` consults; the derivation
// reads linked-issue status, needing no schema change.
func newMilestoneStatusCmd() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "status <milestone-id>",
		Short: "Report a milestone's done-signal (resolved-vs-total linked issues)",
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
				return err
			}

			if flagJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(st)
			}
			cmd.Printf("%s\t%d/%d resolved\tdone=%t\n", st.Milestone, st.Resolved, st.Total, st.Done)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit the done-signal as JSON")
	return cmd
}
