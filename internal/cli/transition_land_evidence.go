package cli

import (
	"encoding/json"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// landEvidence is the issue's verification record, read before any gh call and
// compared to the PR once `gh pr view` has answered. Evidence for another
// commit, lock or branch is void.
type landEvidence struct {
	id, verdict, commit, lock, currentLock, branch string
}

// readLandEvidence refuses verification_missing while the PR is still
// unqueried, so the refusal works offline.
func readLandEvidence(a *core.Artifact, id, project, slug string) (landEvidence, error) {
	verdict, _ := a.FrontMatter["verified_verdict"].(string)
	if verdict == "" {
		return landEvidence{}, errfmt.NewStructured("verification_missing").
			Set("issue", id).
			Set("fix_hint", "run anvil verify "+id+" from the worktree")
	}
	commit, _ := a.FrontMatter["verified_commit"].(string)
	lock, _ := a.FrontMatter["verification_lock"].(string)
	return landEvidence{
		id: id, verdict: verdict, commit: commit, lock: lock,
		currentLock: core.VerificationLock(a.Body),
		branch:      project + "/" + slug,
	}, nil
}

// check reads the PR head once and returns its branch name. A -dirty commit
// never equals a head sha, so it reads as stale. An already-MERGED PR skips the
// verdict checks: refusing after the merge would strand the issue in-progress,
// and the merge already happened. The branch is returned either way.
func (e landEvidence) check(num int, worktreePath string, alreadyMerged bool) (string, error) {
	raw, err := ghPRViewJSONFn(num, "headRefOid,headRefName")
	if err != nil {
		return "", errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	var head struct {
		HeadRefOid  string `json:"headRefOid"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return "", errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	if alreadyMerged {
		return head.HeadRefName, nil
	}
	switch {
	case e.verdict != "pass":
		return "", errfmt.NewStructured("verification_failed").
			Set("issue", e.id).Set("verdict", e.verdict).
			Set("fix_hint", "fix the change, then run anvil verify "+e.id+" until it passes")
	case e.commit != head.HeadRefOid:
		return "", errfmt.NewStructured("verification_stale").
			Set("issue", e.id).Set("pr", num).
			Set("verified_commit", e.commit).Set("pr_head", head.HeadRefOid).
			Set("fix_hint", "push the work, then run anvil verify "+e.id+" at the PR head")
	case e.lock != "" && e.lock != e.currentLock:
		return "", errfmt.NewStructured("verification_changed").
			Set("issue", e.id).
			Set("fix_hint", "review the change, then run anvil verify "+e.id+" --accept-change")
	case !e.ownsBranch(head.HeadRefName, worktreePath):
		return "", errfmt.NewStructured("land_pr_not_issue_pr").
			Set("issue", e.id).Set("pr", num).
			Set("pr_branch", head.HeadRefName).Set("issue_branch", e.branch).
			Set("fix_hint", "pass the PR opened from "+e.branch+", or pass --worktree <path> when the issue worktree uses a renamed branch")
	}
	return head.HeadRefName, nil
}

// ownsBranch accepts the conventional branch, or a renamed one that is checked
// out at the issue's own worktree path (a --branch override or a fleet slug).
func (e landEvidence) ownsBranch(headBranch, worktreePath string) bool {
	if headBranch == e.branch {
		return true
	}
	if headBranch == "" || worktreePath == "" {
		return false
	}
	worktrees, err := gitWorktreeListFn("")
	if err != nil {
		return false
	}
	info, ok := worktrees[headBranch]
	return ok && samePath(info.path, worktreePath)
}
