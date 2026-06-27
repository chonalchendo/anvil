package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// Function-typed package vars let tests swap external command execution
// without shimming binaries on PATH. Pattern mirrors ghPRListFn. The
// gitWorktreeListFn used here is declared once in fleet.go.
var (
	gitWorktreeAddFn       = gitWorktreeAddReal
	gitWorktreeRemoveFn    = gitWorktreeRemoveReal
	gitDeleteLocalBranchFn = gitDeleteLocalBranchReal
	gitMainRootFn          = gitMainRootReal
	gitFetchOriginFn       = gitFetchOriginReal
	gitResolveOriginHEADFn = gitResolveOriginHEADReal
	ghPRViewJSONFn         = ghPRViewJSONReal
	ghPRChecksFn           = ghPRChecksReal
	ghPRMergeFn            = ghPRMergeReal
	ghDeleteBranchFn       = ghDeleteBranchReal
	userHomeFn             = os.UserHomeDir
	mergeabilityPollSleep  = time.Sleep
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

// gitWorktreeAddReal creates a new worktree at path on branch, branching from
// startPoint when non-empty (e.g. "origin/HEAD"). An empty startPoint lets git
// branch from the current HEAD, which is the legacy behaviour.
func gitWorktreeAddReal(repoDir, path, branch, startPoint string) error {
	args := []string{"worktree", "add", path, "-b", branch}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	cmd := exec.Command("git", args...) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitFetchOriginReal() error {
	out, err := exec.Command("git", "fetch", "origin").CombinedOutput() //nolint:gosec // binary path resolved from trusted sources; not user input
	if err != nil {
		return fmt.Errorf("git fetch origin: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitResolveOriginHEADReal resolves the symbolic ref origin/HEAD to a concrete
// remote-tracking ref (e.g. "origin/master"). Returns an error when the remote
// does not exist or has no HEAD — the caller falls back to local HEAD.
func gitResolveOriginHEADReal() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "origin/HEAD").Output() //nolint:gosec // binary path resolved from trusted sources; not user input
	if err != nil {
		return "", fmt.Errorf("git rev-parse origin/HEAD: %w", err)
	}
	ref := strings.TrimSpace(string(out))
	if ref == "" {
		return "", errors.New("git rev-parse origin/HEAD: empty output")
	}
	return ref, nil
}

func gitWorktreeRemoveReal(repoDir, path string) error {
	cmd := exec.Command("git", "worktree", "remove", path) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitDeleteLocalBranchReal deletes the local branch via `git branch -D`, run
// from repoDir (the main worktree) so it does not depend on the caller's cwd —
// which may be the worktree just removed. Run after the worktree is removed so
// git does not refuse the delete on a branch still referenced by a worktree.
// Restores the local-branch cleanup that the old `gh pr merge --delete-branch`
// provided before this path split into a remote-only `gh api` delete.
func gitDeleteLocalBranchReal(repoDir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -D %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// cutWorktreeIfNeeded creates `git worktree add path -b branch [startPoint]`
// unless an entry already matches (idempotent). Errors on path/branch mismatch
// with an existing worktree, or on git failure. Uses fleet.go's
// gitWorktreeListFn (branch-keyed map).
//
// It fetches origin before cutting so the new branch starts from the remote's
// current tip via origin/HEAD rather than a potentially stale local HEAD.
// Offline, no-remote, or an unset origin/HEAD is non-fatal: a warning lands on
// errW (the command's stderr) and the worktree falls back to local HEAD.
func cutWorktreeIfNeeded(errW io.Writer, path, branch string) error {
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
	// Fetch origin so the new branch starts from the remote tip.
	startPoint := ""
	if ferr := gitFetchOriginFn(); ferr != nil {
		fmt.Fprintf(errW, "warning: git fetch origin failed (%v); branching from local HEAD\n", ferr)
	} else if ref, rerr := gitResolveOriginHEADFn(); rerr != nil {
		fmt.Fprintf(errW, "warning: resolving origin/HEAD failed (%v); branching from local HEAD\n", rerr)
	} else {
		startPoint = ref
	}
	return gitWorktreeAddFn("", path, branch, startPoint)
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
	return exec.Command("gh", "pr", "view", strconv.Itoa(num), "--json", fields).Output() //nolint:gosec // binary path resolved from trusted sources; not user input
}

// ghPRChecksReal runs `gh pr checks <num> --required` so optional pending
// checks (e.g. an unfinished CodeRabbit pass) don't gate the merge — only
// the required-checks suite enforces refusal.
func ghPRChecksReal(num int) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "checks", strconv.Itoa(num), "--required") //nolint:gosec // binary path resolved from trusted sources; not user input
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
	// --delete-branch is intentionally omitted: the branch is deleted after the
	// worktree is removed (ghDeleteBranchFn), so git never sees --delete-branch
	// while a worktree still references the branch.
	cmd := exec.Command("gh", "pr", "merge", strconv.Itoa(num), "--squash") //nolint:gosec // binary path resolved from trusted sources; not user input
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh pr merge: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ghDeleteBranchReal deletes the remote branch for the merged PR via the gh
// CLI. The branch name is resolved by the caller (landPR holds it from the
// pre-merge headRefName read), so this no longer re-queries `gh pr view
// headRefName` — post-merge that query can return non-zero once the ref is
// gone, which used to abort the land fatally (anvil.0110). Called after the
// worktree is removed so git doesn't refuse the delete while a worktree still
// references the branch. A ref that is already absent — GitHub's auto-delete of
// merged head branches, or a prior manual delete — is treated as success: the
// post-state is identical.
func ghDeleteBranchReal(branch string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	if branch == "" {
		return errors.New("empty branch name")
	}
	del := exec.Command("gh", "api", "--method", "DELETE", //nolint:gosec // binary path resolved from trusted sources; not user input
		"repos/{owner}/{repo}/git/refs/heads/"+branch)
	if delOut, delErr := del.CombinedOutput(); delErr != nil {
		out := strings.TrimSpace(string(delOut))
		if strings.Contains(out, "Reference does not exist") {
			return nil
		}
		return fmt.Errorf("gh api delete branch %s: %w: %s", branch, delErr, out)
	}
	return nil
}

// doCutWorktree resolves defaults from the issue, applies overrides, and
// cuts the worktree. Returns the worktree path (cut or reused) so the caller
// can emit it — the skill contract is "the claim tells you where to work" —
// or a Structured error on failure so callers can uniformly refuse the
// transition without writing to disk.
func doCutWorktree(errW io.Writer, a *core.Artifact, id, pathOverride, branchOverride string) (path, branch string, err error) {
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	if project == "" || slug == "" {
		return "", "", errfmt.NewStructured("cut_worktree_path_failed").
			Set("error", "issue id lacks `<project>.<slug>` shape").
			Set("id", id)
	}
	wtPath := pathOverride
	if wtPath == "" {
		p, derr := defaultWorktreePath(project, slug)
		if derr != nil {
			return "", "", errfmt.NewStructured("cut_worktree_path_failed").Set("error", derr.Error())
		}
		wtPath = p
	}
	branch = branchOverride
	if branch == "" {
		branch = project + "/" + slug
	}
	if err := cutWorktreeIfNeeded(errW, wtPath, branch); err != nil {
		return "", "", errfmt.NewStructured("cut_worktree_failed").
			Set("path", wtPath).
			Set("branch", branch).
			Set("error", err.Error())
	}
	return wtPath, branch, nil
}

// doLandPR derives the worktree path from the issue and runs landPR. When
// worktreeOverride is non-empty it is used directly; otherwise the path is
// derived from the issue slug. Path derivation is a hard error: the audit line
// claims "worktree removed" and we refuse to lie if we can't compute the
// location.
func doLandPR(errW io.Writer, a *core.Artifact, id string, prNum int, worktreeOverride string, localValidated bool) error {
	if worktreeOverride != "" {
		return landPR(errW, prNum, worktreeOverride, localValidated)
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
	return landPR(errW, prNum, wtPath, localValidated)
}

// landPR runs gate→merge→verify→remove-worktree→delete-local-branch→delete-remote-branch.
// Returns nil on success or a Structured error keyed on the failing gate.
//
// Ordering rationale: the merge runs before the worktree is removed so that
// when the caller's cwd is inside the worktree, the process's working
// directory still exists when gh/git are invoked. The worktree is removed
// after the merge is verified MERGED; both branch deletes follow worktree
// removal because git refuses to delete a branch while a worktree references
// it.
//
// Worktree resolution: if worktreePath exists on disk it is used directly. If
// not, landPR falls back to the live worktree list keyed by the PR's head
// branch — covering the common fleet case where the worktree was cut at a
// non-default slug. If neither path resolves to a real worktree, landPR
// returns land_pr_worktree_missing before merging rather than silently
// skipping removal.
//
// gh pr merge may exit non-zero even when the merge succeeds (post-merge
// checkout fails when master is already checked out by the main worktree), so
// errors from ghPRMergeFn are confirmed against the live PR state before
// propagating.
//
// localValidated skips the ghPRChecks gate when the operator has already
// validated the work locally (e.g. with `just check`) and required CI is
// genuinely unavailable. The caller is responsible for recording an audit line.
func landPR(errW io.Writer, num int, worktreePath string, localValidated bool) error {
	type mergeState struct {
		Mergeable        string `json:"mergeable"`
		MergeStateStatus string `json:"mergeStateStatus"`
	}
	// GitHub recomputes mergeability asynchronously after a sibling PR merges;
	// UNKNOWN means "recomputing, not yet known" — poll until resolved, up to
	// ~30 s. Only CONFLICTING (or any other non-MERGEABLE, non-UNKNOWN value)
	// is a genuine hard abort.
	const maxAttempts = 6
	var st mergeState
	for attempt := range maxAttempts {
		raw, err := ghPRViewJSONFn(num, "mergeable,mergeStateStatus")
		if err != nil {
			return errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
		}
		if err := json.Unmarshal(raw, &st); err != nil {
			return errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
		}
		if st.Mergeable != "UNKNOWN" {
			break
		}
		if attempt < maxAttempts-1 {
			mergeabilityPollSleep(5 * time.Second)
		}
	}
	if st.Mergeable != "MERGEABLE" {
		return errfmt.NewStructured("land_pr_not_mergeable").
			Set("pr", num).
			Set("mergeable", st.Mergeable).
			Set("merge_state", st.MergeStateStatus)
	}
	if !localValidated {
		if err := ghPRChecksFn(num); err != nil {
			return errfmt.NewStructured("land_pr_ci_not_green").Set("pr", num).Set("error", err.Error())
		}
	}
	// Resolve the PR's head branch once: it both keys the worktree-list fallback
	// below and names the local branch to delete after the worktree is removed.
	headBranch := ""
	type headRef struct {
		HeadRefName string `json:"headRefName"`
	}
	var ref headRef
	if raw, rerr := ghPRViewJSONFn(num, "headRefName"); rerr == nil {
		_ = json.Unmarshal(raw, &ref)
	}
	headBranch = ref.HeadRefName
	// Resolve the actual worktree path: try the explicit/default path first,
	// then fall back to the live worktree list keyed by the PR's head branch.
	resolved := ""
	if worktreePath != "" {
		if _, ferr := os.Stat(worktreePath); ferr == nil {
			resolved = worktreePath
		}
	}
	if resolved == "" && headBranch != "" {
		if worktrees, lerr := gitWorktreeListFn(); lerr == nil {
			if info, ok := worktrees[headBranch]; ok {
				resolved = info.path
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
	// Move the process cwd to root before removing the worktree: --land-pr is
	// naturally fired from inside the issue worktree, and substeps that inherit
	// cwd (ghDeleteBranchFn, the post-doLandPR open-PR check) would otherwise
	// shell out from the just-deleted directory and fail with a getwd error.
	if cherr := os.Chdir(root); cherr != nil {
		return errfmt.NewStructured("land_pr_chdir_root_failed").Set("root", root).Set("error", cherr.Error())
	}
	// Merge before removing the worktree so the process's cwd remains valid
	// throughout. gh pr merge may exit non-zero even when the merge lands
	// (post-merge checkout fails when master is checked out in the main
	// worktree), so we confirm the state rather than trusting the exit code.
	mergeErr := ghPRMergeFn(num)
	type finalState struct {
		State string `json:"state"`
	}
	raw, err := ghPRViewJSONFn(num, "state")
	if err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	var fin finalState
	if err := json.Unmarshal(raw, &fin); err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	if fin.State != "MERGED" {
		// Surface the merge error when available; fall back to state mismatch.
		if mergeErr != nil {
			return errfmt.NewStructured("land_pr_merge_failed").Set("pr", num).Set("error", mergeErr.Error())
		}
		return errfmt.NewStructured("land_pr_state_not_merged").Set("pr", num).Set("state", fin.State)
	}
	// Worktree removal stays fatal: a failure here usually means uncommitted
	// work in the worktree, and surfacing it lets the operator recover that work
	// rather than silently discard it. The branches, by contrast, only name a
	// ref whose content is already merged.
	if err := gitWorktreeRemoveFn(root, resolved); err != nil {
		return errfmt.NewStructured("land_pr_worktree_remove_failed").
			Set("pr", num).
			Set("path", resolved).
			Set("error", err.Error())
	}
	// The branch deletes are best-effort cleanup of a now-redundant ref: each
	// failure is warned and skipped, never returned fatally, so a confirmed
	// MERGED PR can never strand the issue in-progress (anvil.0110). The
	// local-branch delete runs after worktree removal (git refuses to delete a
	// branch a worktree still references); both restore the parity the old
	// `gh pr merge --delete-branch` gave — a successful land leaves neither a
	// local nor a remote branch behind.
	if headBranch != "" {
		if err := gitDeleteLocalBranchFn(root, headBranch); err != nil {
			fmt.Fprintf(errW, "warning: land-pr %d: local branch delete failed (%s): %v\n", num, headBranch, err)
		}
		if err := ghDeleteBranchFn(headBranch); err != nil {
			fmt.Fprintf(errW, "warning: land-pr %d: remote branch delete failed (%s): %v\n", num, headBranch, err)
		}
	} else {
		fmt.Fprintf(errW, "warning: land-pr %d: head branch unresolved; skipping branch cleanup\n", num)
	}
	return nil
}
