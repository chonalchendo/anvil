package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// landEvidence is what the land verb knows of the issue before `gh pr view`
// answers: its claim lock, the branch a PR must come from, and the body's
// review rounds. Evidence for another lock or branch is void. The verdict is
// not read from the issue: the
// land earns its own on a clean checkout of the PR head.
type landEvidence struct {
	id, lock, currentLock, branch, body string
}

func newLandEvidence(a *core.Artifact, id, project, slug string) landEvidence {
	lock, _ := a.FrontMatter["verification_lock"].(string)
	return landEvidence{
		id: id, lock: lock, currentLock: core.VerificationLock(a.Body),
		branch: project + "/" + slug, body: a.Body,
	}
}

// landClean is what the clean run needs: the in-memory issue (whose body is
// verified and which takes the stamp back), its id, and the vault to index in.
type landClean struct {
	a  *core.Artifact
	v  *core.Vault
	id string
}

// landHead is the PR head the evidence check read.
type landHead struct{ oid, branch string }

// check reads the PR head once. An already-MERGED PR skips the lock,
// ownership and review checks: refusing after the merge would strand the issue
// in-progress, and the merge already happened. The head is returned either way.
func (e landEvidence) check(num int, worktreePath string, alreadyMerged bool) (landHead, error) {
	raw, err := ghPRViewJSONFn(num, "headRefOid,headRefName")
	if err != nil {
		return landHead{}, errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	var head struct {
		HeadRefOid  string `json:"headRefOid"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return landHead{}, errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	h := landHead{oid: head.HeadRefOid, branch: head.HeadRefName}
	if alreadyMerged {
		return h, nil
	}
	// The round's sha is a prefix match: the round records the abbreviated head.
	rr, ok := latestReviewRound(e.body, num)
	switch {
	case e.lock != "" && e.lock != e.currentLock:
		return landHead{}, errfmt.NewStructured("verification_changed").
			Set("issue", e.id).
			Set("fix_hint", "review the change, then run anvil verify "+e.id+" --accept-change")
	case !e.ownsBranch(head.HeadRefName, worktreePath):
		return landHead{}, errfmt.NewStructured("land_pr_not_issue_pr").
			Set("issue", e.id).Set("pr", num).
			Set("pr_branch", head.HeadRefName).Set("issue_branch", e.branch).
			Set("fix_hint", "pass the PR opened from "+e.branch+", or pass --worktree <path> when the issue worktree uses a renamed branch")
	case !ok:
		return landHead{}, errfmt.NewStructured("land_pr_review_missing").
			Set("issue", e.id).Set("pr", num).
			Set("fix_hint", "fire reviewing-pr on the PR; it persists the round on the issue")
	case !strings.HasPrefix(h.oid, rr.sha):
		return landHead{}, errfmt.NewStructured("land_pr_review_stale").
			Set("reviewed", rr.sha).Set("head", h.oid).
			Set("fix_hint", "fire a fresh reviewing-pr round at the PR head")
	case rr.blocked:
		return landHead{}, errfmt.NewStructured("land_pr_review_blocked").
			Set("round", rr.round).
			Set("fix_hint", "dispatch the responder, then a confirming reviewing-pr round")
	}
	return h, nil
}

// landCleanRunFn is a seam: land tests that stop short of verification stub it.
var landCleanRunFn = landCleanRun

// landCleanRun verifies the PR head on a fresh detached checkout and stamps the
// record at that sha, pass or fail. A red run refuses before the merge. A failed
// fetch is a notice: the sha may already be local, and checkoutAt refuses if not.
func landCleanRun(errW io.Writer, c landClean, root, sha string) error {
	fetchErr := gitFetchOriginFn(root)
	if fetchErr != nil {
		fmt.Fprintf(errW, "warning: land-pr: fetch failed, using local refs: %v\n", fetchErr)
	}
	rec, err := verifyAt(errW, c.id, root, sha, c.a.Body)
	if err != nil {
		var se *errfmt.Structured
		if fetchErr != nil && errors.As(err, &se) && se.Code == "verify_at_unresolved" {
			return se.Set("fetch_error", fetchErr.Error())
		}
		return err
	}
	if err := stampVerification(c.v, c.a.Path, c.id, c.id, rec, ""); err != nil {
		return err
	}
	// The command's single Save() writes c.a: carry the stamp onto it, or that
	// save would erase what the clean run just wrote.
	c.a.FrontMatter["verified_verdict"] = rec.Verdict
	c.a.FrontMatter["verified_commit"] = rec.Commit
	c.a.FrontMatter["verified_at"] = rec.RanAt
	if rec.Verdict == "pass" {
		return nil
	}
	checks := make([]string, len(rec.Failed))
	for i, f := range rec.Failed {
		checks[i] = f.Check
	}
	return errfmt.NewStructured("land_pr_verification_failed").
		Set("issue", c.id).Set("commit", rec.Commit).Set("failed", checks).
		Set("fix_hint", "fix the change and push, then re-run --land-pr; the clean run is at "+rec.Commit)
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
