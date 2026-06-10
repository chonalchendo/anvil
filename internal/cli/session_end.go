package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newSessionEndCmd() *cobra.Command {
	var flagCommit bool
	cmd := &cobra.Command{
		Use:   "end",
		Short: "End-of-session cleanup: optionally snapshot uncommitted vault artifacts",
		Args:  cobra.NoArgs,
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
			if err := gitRun(v.Root, "add", "-A"); err != nil {
				return fmt.Errorf("git add: %w", err)
			}
			msg := "anvil vault snapshot: " + time.Now().UTC().Format(time.RFC3339)
			if err := gitRun(v.Root, "commit", "-m", msg); err != nil {
				return fmt.Errorf("git commit: %w", err)
			}
			cmd.Printf("committed %d change(s) to the vault\n", st.Dirty)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagCommit, "commit", false, "snapshot uncommitted vault artifacts with git")
	return cmd
}
