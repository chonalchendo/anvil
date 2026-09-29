package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	var flagPush bool
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Snapshot this session's vault changes with git (peer sessions' files held back)",
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
			return snapshotVault(cmd, v.Root, flagMessage, st, flagPush)
		},
	}
	cmd.Flags().StringVarP(&flagMessage, "message", "m", "", "commit message (default: timestamped snapshot)")
	cmd.Flags().BoolVar(&flagPush, "push", false, "push to the vault's remote after committing (warns, never fails, on push error or no remote)")
	return cmd
}

// snapshotVault commits the vault's pending changes except other sessions'
// files under 10-sessions/, which a concurrent peer may still be writing; those
// are named on stderr and left dirty. Callers decide the not-repo/clean-tree
// policy first; an empty msg gets the timestamped default. When push is set the
// commit is pushed — a no-op when st has no remote, and a stderr warning (never
// a returned error) on push failure, so a missing network never breaks teardown.
func snapshotVault(cmd *cobra.Command, root, msg string, st core.VaultGitStatus, push bool) error {
	if msg == "" {
		msg = "anvil vault snapshot: " + time.Now().UTC().Format(time.RFC3339)
	}
	out, err := gitOutput(root, "ls-files", "-z", "-m", "-o", "-d", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	own := ownSessionRelPath(root)
	var mine, held []string
	seen := map[string]bool{}
	for _, p := range strings.Split(out, "\x00") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if strings.HasPrefix(p, "10-sessions/") && p != own {
			held = append(held, p)
		} else {
			mine = append(mine, p)
		}
	}
	if len(held) > 0 {
		cmd.PrintErrln("held back (another session's files, left uncommitted):", strings.Join(held, ", "))
	}
	if len(mine) == 0 {
		cmd.Println("nothing of this session's to commit")
		return nil
	}
	if err := gitRun(root, append([]string{"add", "--"}, mine...)...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	// Pathspec commit ignores anything a peer staged in the shared index.
	if err := gitRun(root, append([]string{"commit", "-m", msg, "--"}, mine...)...); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	cmd.Printf("committed %d change(s) to the vault\n", len(mine))
	if push && st.HasRemote {
		if err := gitRun(root, "push"); err != nil {
			cmd.PrintErrln("⚠ vault push failed (commit is safe locally):", err)
			return nil
		}
		cmd.Println("pushed the vault to its remote")
	}
	return nil
}

// ownSessionRelPath is the calling session's file relative to root, or "" when
// no session is resolvable (then every 10-sessions/ file counts as a peer's).
func ownSessionRelPath(root string) string {
	id := os.Getenv(envSessionID)
	if id == "" {
		var err error
		if id, err = codexSessionID(); err != nil {
			return ""
		}
	}
	rel, err := filepath.Rel(root, core.TypeSession.Path(root, id))
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

func gitOutput(dir string, args ...string) (string, error) {
	c := exec.Command("git", args...) //nolint:gosec // G204: args are package-internal literals, never user input
	c.Dir = dir
	out, err := c.Output()
	return string(out), err
}

func gitRun(dir string, args ...string) error {
	c := exec.Command("git", args...) //nolint:gosec // G204: args are package-internal literals, never user input
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
