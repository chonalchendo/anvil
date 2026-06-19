package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// newNextCmd is the deterministic front-door for "what should I work on next":
// it returns the single highest-priority ready issue plus the start-context an
// agent needs to begin it (goal, severity, milestone, governing contracts,
// path). The selection and ordering are shared with `anvil build` (via
// selectReadyUnits) so an interactive agent and the build loop agree on the
// same next unit.
func newNextCmd() *cobra.Command {
	var (
		flagJSON      bool
		flagProject   string
		flagMilestone string
	)

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Print the next ready issue plus its start-context",
		Args:  cobra.NoArgs,
		Example: `  anvil next --json
  anvil next --project anvil --milestone anvil.<slug> --json`,
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
			units := selectReadyUnits(rows, flagMilestone)

			if flagJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if len(units) == 0 {
					// Empty object, not null: a deterministic "no work" shape
					// callers can branch on without a special exit code.
					return enc.Encode(struct{}{})
				}
				u := units[0]
				if u.Contracts == nil {
					u.Contracts = []string{}
				}
				return enc.Encode(u)
			}

			if len(units) == 0 {
				cmd.PrintErrln("no ready issues")
				return nil
			}
			cmd.Printf("%s\t%s\n", units[0].ID, units[0].Goal)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit the next unit as a JSON start-context object")
	cmd.Flags().StringVar(&flagProject, "project", "", "restrict to ready issues in this project (exact match; default: all)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "restrict to ready issues under this milestone slug")
	return cmd
}
