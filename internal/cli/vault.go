package cli

import (
	"fmt"
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
			return snapshotVault(cmd, v.Root, flagMessage, st, flagPush, ownSessionID())
		},
	}
	cmd.Flags().StringVarP(&flagMessage, "message", "m", "", "commit message (default: timestamped snapshot)")
	cmd.Flags().BoolVar(&flagPush, "push", false, "push to the vault's remote after committing (warns, never fails, on push error or no remote)")
	return cmd
}

// snapshotVault commits the vault's pending changes except a concurrent peer's
// in-flight files: other sessions' stubs under the sessions dir and in-progress
// issues claimed by another session. Held files are named on stderr and left
// dirty; deletions under the sessions dir are always committed (a deleted file
// is not in-flight, and no other verb would ever commit a gc'd stub). ownID is
// the caller's resolved session id, "" when none — then every session file and
// every claimed issue is held. Callers decide the not-repo/clean-tree policy
// first; an empty msg gets the timestamped default. When push is set the commit
// is pushed — a no-op when st has no remote, and a stderr warning (never a
// returned error) on push failure, so a missing network never breaks teardown.
func snapshotVault(cmd *cobra.Command, root, msg string, st core.VaultGitStatus, push bool, ownID string) error {
	if msg == "" {
		msg = "anvil vault snapshot: " + time.Now().UTC().Format(time.RFC3339)
	}
	out, err := gitOutput(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	mine, held := partitionVaultChanges(root, out, ownID)
	if len(held) > 0 {
		label := "held back (another session's files, left uncommitted):"
		if ownID == "" {
			label = "held back (no session id resolved; session files left uncommitted):"
		}
		cmd.PrintErrln(label, strings.Join(held, ", "))
	}
	if len(mine) == 0 {
		cmd.Println("nothing of this session's to commit")
		return nil
	}
	paths := strings.Join(mine, "\x00") + "\x00"
	if err := gitRunStdin(root, paths, "--literal-pathspecs", "add", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	// Pathspec commit ignores anything a peer staged in the shared index.
	if err := gitRunStdin(root, paths, "--literal-pathspecs", "commit", "-m", msg, "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
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

// partitionVaultChanges splits `git status --porcelain=v1 -z` output into the
// paths this session may commit and the peer-owned ones to hold back. Status
// (not ls-files) is the source so staged-only changes are seen.
func partitionVaultChanges(root, porcelain, ownID string) (mine, held []string) {
	sessionsPrefix := core.TypeSession.Dir() + "/"
	ownPath := ""
	if ownID != "" {
		ownPath = sessionsPrefix + ownID + ".md"
	}
	entries := strings.Split(porcelain, "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		xy, p := e[:2], e[3:]
		if strings.ContainsAny(xy, "RC") && i+1 < len(entries) {
			// Rename/copy lists the origin as the next entry; commit both ends.
			i++
			mine = append(mine, entries[i])
		}
		deleted := strings.Contains(xy, "D")
		switch {
		case strings.HasPrefix(p, sessionsPrefix) && p != ownPath && !deleted:
			held = append(held, p)
		case strings.HasPrefix(p, core.TypeIssue.Dir()+"/") && !deleted && claimedByPeer(root, p, ownID):
			held = append(held, p)
		default:
			mine = append(mine, p)
		}
	}
	return mine, held
}

// claimedByPeer reports whether the in-progress issue at rel carries a
// claim_session other than ownID. Unreadable files are the caller's own.
func claimedByPeer(root, rel, ownID string) bool {
	a, err := core.LoadArtifact(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	if status, _ := a.FrontMatter["status"].(string); status != "in-progress" {
		return false
	}
	claim, _ := a.FrontMatter["claim_session"].(string)
	return claim != "" && claim != ownID
}

// ownSessionID resolves the calling session for snapshot scope, or "" when
// unresolvable. The Codex newest-rollout guess is deliberately rejected: from a
// plain terminal it would name a live Codex session and commit its in-flight
// file as this caller's own.
func ownSessionID() string {
	id, _, src, err := resolveCurrentSession()
	if err != nil || src != "claude-code" {
		return ""
	}
	return id
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

func gitRunStdin(dir, stdin string, args ...string) error {
	c := exec.Command("git", args...) //nolint:gosec // G204: args are package-internal literals, never user input
	c.Dir = dir
	c.Stdin = strings.NewReader(stdin)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
