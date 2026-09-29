package cli

import (
	"fmt"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// checkLiveOpenIssue flags an open issue that already carries a worktree or
// branch named for its slug: someone is working it with no claim recorded.
// Offline by design (local git state only). The fix never removes the
// worktree — it is presumed live, and removing it wedges the running agent.
func checkLiveOpenIssue(id string, a *core.Artifact, worktrees map[string]worktreeInfo) *doctorFinding {
	slug := slugFromIssueID(id)
	evidence := ""
	for branch, wt := range worktrees {
		switch {
		case worktreePathMatchesIssue(map[string]worktreeInfo{branch: wt}, id):
			evidence = fmt.Sprintf("worktree %s (branch %s)", wt.path, branch)
		case strings.HasSuffix(branch, "/"+slug):
			evidence = fmt.Sprintf("branch %s", branch)
		}
		if evidence != "" {
			break
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
