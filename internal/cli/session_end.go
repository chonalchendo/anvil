package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newSessionEndCmd() *cobra.Command {
	var flagCommit bool
	var flagPush bool
	cmd := &cobra.Command{
		Use:     "end",
		Short:   "End-of-session cleanup: optionally snapshot uncommitted vault artifacts",
		Example: "  anvil session end --commit --push",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !flagCommit {
				return nil
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			st, err := core.VaultGitState(v.Root)
			if err != nil {
				return err
			}
			if st.NotRepo || st.Dirty == 0 {
				return nil
			}
			return snapshotVault(cmd, v.Root, "", st, flagPush)
		},
	}
	cmd.Flags().BoolVar(&flagCommit, "commit", false, "snapshot uncommitted vault artifacts with git")
	cmd.Flags().BoolVar(&flagPush, "push", false, "push to the vault's remote after committing (requires --commit; warns, never fails, on push error)")
	return cmd
}
