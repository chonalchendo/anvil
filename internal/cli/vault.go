package cli

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Vault-level version-control maintenance",
	}
	cmd.AddCommand(newVaultCommitCmd())
	return cmd
}

func newVaultCommitCmd() *cobra.Command {
	var flagMessage string
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Snapshot the whole vault with git (add -A + commit)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			st, err := core.VaultGitState(v.Root)
			if err != nil {
				return err
			}
			if st.NotRepo {
				return fmt.Errorf("vault at %s is not a git repo; run `git init` there first", v.Root)
			}
			if st.Dirty == 0 {
				cmd.Println("vault clean — nothing to commit")
				return nil
			}
			return snapshotVault(cmd, v.Root, flagMessage, st.Dirty)
		},
	}
	cmd.Flags().StringVarP(&flagMessage, "message", "m", "", "commit message (default: timestamped snapshot)")
	return cmd
}

// snapshotVault stages and commits every pending change under root, printing
// the committed count. Callers decide the not-repo/clean-tree policy before
// calling; an empty msg gets the timestamped default.
func snapshotVault(cmd *cobra.Command, root, msg string, dirty int) error {
	if msg == "" {
		msg = "anvil vault snapshot: " + time.Now().UTC().Format(time.RFC3339)
	}
	if err := gitRun(root, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := gitRun(root, "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	cmd.Printf("committed %d change(s) to the vault\n", dirty)
	return nil
}

func gitRun(dir string, args ...string) error {
	c := exec.Command("git", args...) //nolint:gosec // G204: args are package-internal literals, never user input
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
