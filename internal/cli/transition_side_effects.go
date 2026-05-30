package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// Function-typed package vars let tests swap external command execution
// without shimming binaries on PATH. Pattern mirrors ghPRListFn. The
// gitWorktreeListFn used here is declared once in fleet.go.
var (
	gitWorktreeAddFn    = gitWorktreeAddReal
	gitWorktreeRemoveFn = gitWorktreeRemoveReal
	gitMainRootFn       = gitMainRootReal
	ghPRViewJSONFn      = ghPRViewJSONReal
	ghPRChecksFn        = ghPRChecksReal
	ghPRMergeFn         = ghPRMergeReal
	userHomeFn          = os.UserHomeDir
)

// claimConflict reports whether claiming `a` (issue → in-progress) collides with
// an existing claim held by a different session under the same owner. It returns
// a Structured refusal when the issue already records a `claim_session` that
// disagrees with the invoking session, naming the holder so the agent can see
// which session to coordinate with. A claim from the same session, an unclaimed
// issue, or an invocation outside a Claude session (no CLAUDE_CODE_SESSION_ID)
// is not a conflict — session-keyed exclusivity only applies when both the
// holder and the claimant carry a session id.
func claimConflict(a *core.Artifact, id, currentSession string) error {
	held, _ := a.FrontMatter["claim_session"].(string)
	if held == "" || currentSession == "" || held == currentSession {
		return nil
	}
	return errfmt.NewStructured("claim_held_by_other_session").
		Set("issue", id).
		Set("holding_session", held).
		Set("this_session", currentSession).
		Set("fix_hint", "another session is already working this issue; coordinate or rerun with --force to take over the claim")
}

func projectFromArtifact(a *core.Artifact, id string) string {
	if p, _ := a.FrontMatter["project"].(string); p != "" {
		return p
	}
	if dot := strings.IndexByte(id, '.'); dot > 0 {
		return id[:dot]
	}
	return ""
}

func slugFromIssueID(id string) string {
	dot := strings.IndexByte(id, '.')
	if dot < 0 || dot+1 >= len(id) {
		return id
	}
	return id[dot+1:]
}

func defaultWorktreePath(project, slug string) (string, error) {
	home, err := userHomeFn()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Development", project+"-worktrees", slug), nil
}

func gitWorktreeAddReal(repoDir, path, branch string) error {
	cmd := exec.Command("git", "worktree", "add", path, "-b", branch)
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitWorktreeRemoveReal(repoDir, path string) error {
	cmd := exec.Command("git", "worktree", "remove", path)
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// cutWorktreeIfNeeded creates `git worktree add path -b branch` unless an
// entry already matches (idempotent). Errors on path/branch mismatch with an
// existing worktree, or on git failure. Uses fleet.go's gitWorktreeListFn
// (branch-keyed map).
func cutWorktreeIfNeeded(path, branch string) error {
	worktrees, err := gitWorktreeListFn()
	if err != nil {
		return err
	}
	if info, ok := worktrees[branch]; ok {
		if info.path == path {
			return nil
		}
		return fmt.Errorf("branch %q already checked out at %s (expected %s)", branch, info.path, path)
	}
	for b, info := range worktrees {
		if info.path == path {
			return fmt.Errorf("worktree at %s already on branch %q (expected %q)", path, b, branch)
		}
	}
	return gitWorktreeAddFn("", path, branch)
}

// gitMainRootReal returns the main worktree's root directory by deriving it
// from `git rev-parse --git-common-dir`. The common dir is `<main>/.git` for
// every non-bare checkout, so its parent is the main worktree. Used as a
// stable cwd for `git worktree remove` when the caller's cwd is the worktree
// being removed.
func gitMainRootReal() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return "", errors.New("git rev-parse --git-common-dir: empty output")
	}
	return filepath.Dir(common), nil
}

func ghPRViewJSONReal(num int, fields string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errGhUnavailable
	}
	return exec.Command("gh", "pr", "view", strconv.Itoa(num), "--json", fields).Output()
}

// ghPRChecksReal runs `gh pr checks <num> --required` so optional pending
// checks (e.g. an unfinished CodeRabbit pass) don't gate the merge — only
// the required-checks suite enforces refusal.
func ghPRChecksReal(num int) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "checks", strconv.Itoa(num), "--required")
	out, err := cmd.CombinedOutput()
	return classifyPRChecks(string(out), err)
}

// classifyPRChecks interprets the result of `gh pr checks --required`. gh exits
// non-zero with "no required checks reported" when the branch configures zero
// required checks — there is nothing to gate, so that is green, not a failure.
// Any other non-nil err refuses the land.
func classifyPRChecks(out string, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(out, "no required checks reported") {
		return nil
	}
	return fmt.Errorf("gh pr checks: %w: %s", err, strings.TrimSpace(out))
}

func ghPRMergeReal(num int) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "merge", strconv.Itoa(num), "--squash", "--delete-branch")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh pr merge: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// doCutWorktree resolves defaults from the issue, applies overrides, and
// cuts the worktree. Returns a Structured error on failure so callers can
// uniformly refuse the transition without writing to disk.
func doCutWorktree(a *core.Artifact, id, pathOverride, branchOverride string) error {
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	if project == "" || slug == "" {
		return errfmt.NewStructured("cut_worktree_path_failed").
			Set("error", "issue id lacks `<project>.<slug>` shape").
			Set("id", id)
	}
	wtPath := pathOverride
	if wtPath == "" {
		p, derr := defaultWorktreePath(project, slug)
		if derr != nil {
			return errfmt.NewStructured("cut_worktree_path_failed").Set("error", derr.Error())
		}
		wtPath = p
	}
	branch := branchOverride
	if branch == "" {
		branch = project + "/" + slug
	}
	if err := cutWorktreeIfNeeded(wtPath, branch); err != nil {
		return errfmt.NewStructured("cut_worktree_failed").
			Set("path", wtPath).
			Set("branch", branch).
			Set("error", err.Error())
	}
	return nil
}

// doLandPR derives the worktree path from the issue and runs landPR. When
// worktreeOverride is non-empty it is used directly; otherwise the path is
// derived from the issue slug. Path derivation is a hard error: the audit line
// claims "worktree removed" and we refuse to lie if we can't compute the
// location.
func doLandPR(a *core.Artifact, id string, prNum int, worktreeOverride string) error {
	if worktreeOverride != "" {
		return landPR(prNum, worktreeOverride)
	}
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	if project == "" || slug == "" {
		return errfmt.NewStructured("land_pr_path_failed").
			Set("error", "issue id lacks `<project>.<slug>` shape").
			Set("id", id)
	}
	wtPath, derr := defaultWorktreePath(project, slug)
	if derr != nil {
		return errfmt.NewStructured("land_pr_path_failed").Set("error", derr.Error())
	}
	return landPR(prNum, wtPath)
}

// landPR runs gate→remove→merge→verify. Returns nil on success or a Structured
// error keyed on the failing gate. Order matches docs/worktree-workflow.md:
// the worktree must be removed before `gh pr merge --delete-branch` will land
// the branch delete (gh refuses the delete while a worktree references it).
//
// Worktree resolution: if worktreePath exists on disk it is used directly. If
// not, landPR falls back to the live worktree list keyed by the PR's head
// branch — covering the common fleet case where the worktree was cut at a
// non-default slug. If neither path resolves to a real worktree, landPR
// returns land_pr_worktree_missing before merging rather than silently
// skipping removal and letting --delete-branch fail mid-flight.
func landPR(num int, worktreePath string) error {
	type mergeState struct {
		Mergeable        string `json:"mergeable"`
		MergeStateStatus string `json:"mergeStateStatus"`
	}
	raw, err := ghPRViewJSONFn(num, "mergeable,mergeStateStatus")
	if err != nil {
		return errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	var st mergeState
	if err := json.Unmarshal(raw, &st); err != nil {
		return errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	if st.Mergeable != "MERGEABLE" {
		return errfmt.NewStructured("land_pr_not_mergeable").
			Set("pr", num).
			Set("mergeable", st.Mergeable).
			Set("merge_state", st.MergeStateStatus)
	}
	if err := ghPRChecksFn(num); err != nil {
		return errfmt.NewStructured("land_pr_ci_not_green").Set("pr", num).Set("error", err.Error())
	}
	// Resolve the actual worktree path: try the explicit/default path first,
	// then fall back to the live worktree list keyed by the PR's head branch.
	resolved := ""
	if worktreePath != "" {
		if _, ferr := os.Stat(worktreePath); ferr == nil {
			resolved = worktreePath
		}
	}
	if resolved == "" {
		// Derive worktree from PR's head branch via the live worktree list.
		type headRef struct {
			HeadRefName string `json:"headRefName"`
		}
		var ref headRef
		if raw, rerr := ghPRViewJSONFn(num, "headRefName"); rerr == nil {
			_ = json.Unmarshal(raw, &ref)
		}
		if ref.HeadRefName != "" {
			if worktrees, lerr := gitWorktreeListFn(); lerr == nil {
				if info, ok := worktrees[ref.HeadRefName]; ok {
					resolved = info.path
				}
			}
		}
	}
	if resolved == "" {
		// No worktree found via either path: error before merging so the
		// caller can supply --worktree and retry without a half-complete state.
		return errfmt.NewStructured("land_pr_worktree_missing").
			Set("pr", num).
			Set("tried_path", worktreePath).
			Set("fix_hint", "pass --worktree <path> to point --land-pr at the actual worktree, or remove it manually then retry")
	}
	root, rerr := gitMainRootFn()
	if rerr != nil {
		return errfmt.NewStructured("land_pr_main_worktree_lookup_failed").Set("error", rerr.Error())
	}
	if err := gitWorktreeRemoveFn(root, resolved); err != nil {
		return errfmt.NewStructured("land_pr_worktree_remove_failed").
			Set("pr", num).
			Set("path", resolved).
			Set("error", err.Error())
	}
	if err := ghPRMergeFn(num); err != nil {
		return errfmt.NewStructured("land_pr_merge_failed").Set("pr", num).Set("error", err.Error())
	}
	type finalState struct {
		State string `json:"state"`
	}
	raw, err = ghPRViewJSONFn(num, "state")
	if err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	var fin finalState
	if err := json.Unmarshal(raw, &fin); err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	if fin.State != "MERGED" {
		return errfmt.NewStructured("land_pr_state_not_merged").Set("pr", num).Set("state", fin.State)
	}
	return nil
}
