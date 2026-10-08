package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitLocalBranchesFn lists local and remote-tracking branch names from the
// cwd repo's existing refs. Injectable so tests don't shell out.
var gitLocalBranchesFn = gitLocalBranchesReal

// gitLocalBranchesReal reads refs only — no fetch, no network — so doctor
// stays offline.
func gitLocalBranchesReal() ([]string, error) {
	out, err := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes").Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// checkLiveOpenIssue flags an open issue that already carries a worktree or
// branch named for its slug: someone is working it with no claim recorded.
// Offline by design (local git state only). The fix never removes the
// worktree — it is presumed live, and removing it wedges the running agent.
func checkLiveOpenIssue(id string, worktrees map[string]worktreeInfo, branches []string) *doctorFinding {
	slug := slugFromIssueID(id)
	evidence := ""
	for branch, wt := range worktrees {
		if filepath.Base(wt.path) == slug {
			evidence = fmt.Sprintf("worktree %s (branch %s)", wt.path, branch)
			break
		}
	}
	if evidence == "" {
		for _, b := range branches {
			if strings.HasSuffix(b, "/"+slug) {
				evidence = fmt.Sprintf("branch %s", b)
				break
			}
		}
	}
	if evidence == "" {
		return nil
	}
	return &doctorFinding{
		Kind:     "live-work-on-open",
		ID:       id,
		Evidence: fmt.Sprintf("status is open but live work exists: %s", evidence),
		Fix:      fmt.Sprintf("confirm who owns the worktree, then claim it: anvil transition issue %s in-progress --owner <name> (do not remove the worktree)", id),
	}
}
