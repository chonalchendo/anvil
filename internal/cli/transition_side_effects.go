package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	resolveProjectRepoFn   = resolveProjectRepoReal
	gitToplevelFn          = gitToplevelReal
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

var outcomeLinkRe = regexp.MustCompile(`\[\[((?:decision|learning)\.[^\]]+)\]\]`)

// threadOutcomeLinked reports whether a closing thread routes its outcome to a
// decision or learning that exists. Both the body (hand-authored resolution
// prose) and `related` — the only link slot thread.schema.json allows, and
// where `anvil link` writes — count, so a properly-linked thread never draws
// the warning. Fenced blocks are stripped (an illustrative wikilink in a code
// sample is not an outcome) and a dangling target does not count: a typo'd id
// must not silence the gate.
func threadOutcomeLinked(v *core.Vault, a *core.Artifact) bool {
	if outcomeLinkResolves(v, core.StripFencedBlocks(a.Body)) {
		return true
	}
	vals, _ := a.FrontMatter["related"].([]any)
	for _, e := range vals {
		if s, ok := e.(string); ok && outcomeLinkResolves(v, s) {
			return true
		}
	}
	return false
}

// outcomeLinkResolves reports whether s carries a decision or learning wikilink
// whose target names a file present in v.
func outcomeLinkResolves(v *core.Vault, s string) bool {
	for _, m := range outcomeLinkRe.FindAllStringSubmatch(s, -1) {
		if core.WikilinkTargetExists(v, m[1]) {
			return true
		}
	}
	return false
}

func projectFromArtifact(a *core.Artifact, id string) string {
	if p, _ := a.FrontMatter["project"].(string); p != "" {
		return p
	}
	bare := core.BareID(core.TypeIssue, id)
	if dot := strings.IndexByte(bare, '.'); dot > 0 {
		return bare[:dot]
	}
	return ""
}

// slugFromIssueID returns the `<ordinal>.<slug>` tail that names a worktree
// directory and branch.
func slugFromIssueID(id string) string {
	id = core.BareID(core.TypeIssue, id)
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

// cutWorktreeIfNeeded creates `git worktree add path -b branch [startPoint]`
// from repoDir unless an entry already matches (idempotent). Errors on
// path/branch mismatch with an existing worktree, or on git failure. Uses
// fleet.go's gitWorktreeListFn (branch-keyed map).
//
// It fetches origin (from repoDir) before cutting so the new branch starts
// from the remote's current tip via origin/HEAD rather than a potentially
// stale local HEAD. Offline, no-remote, or an unset origin/HEAD is non-fatal:
// a warning lands on errW (the command's stderr) and the worktree falls back
// to local HEAD.
func cutWorktreeIfNeeded(errW io.Writer, repoDir, path, branch string) error {
	worktrees, err := gitWorktreeListFn(repoDir)
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
	if ferr := gitFetchOriginFn(repoDir); ferr != nil {
		fmt.Fprintf(errW, "warning: git fetch origin failed (%v); branching from local HEAD\n", ferr)
	} else if ref, rerr := gitResolveOriginHEADFn(repoDir); rerr != nil {
		fmt.Fprintf(errW, "warning: resolving origin/HEAD failed (%v); branching from local HEAD\n", rerr)
	} else {
		startPoint = ref
	}
	return gitWorktreeAddFn(repoDir, path, branch, startPoint)
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
	repoDir, rerr := resolveProjectRepoFn(project)
	if rerr != nil {
		return "", "", errfmt.NewStructured("cut_worktree_repo_unresolved").
			Set("project", project).
			Set("error", rerr.Error())
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
	if err := cutWorktreeIfNeeded(errW, repoDir, wtPath, branch); err != nil {
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
// The ordering, merge-exit-code, and worktree-resolution rationale live as
// comments at each step.
//
// Worktree resolution: if worktreePath exists on disk it is used directly. If
// not, landPR falls back to the live worktree list keyed by the PR's head
// branch — covering the common fleet case where the worktree was cut at a
// non-default slug. If neither path resolves to a real worktree, landPR
// returns land_pr_worktree_missing before merging rather than silently
// skipping removal — unless the PR is already MERGED (a retry of an
// interrupted land), where a missing worktree is treated as already cleaned.
//
// localValidated skips the ghPRChecks gate when the operator has already
// validated the work locally (e.g. with `just check`) and required CI is
// genuinely unavailable. The caller is responsible for recording an audit line.
func landPR(errW io.Writer, num int, worktreePath string, localValidated bool) error {
	// An already-MERGED PR also reads mergeable:UNKNOWN (GitHub never
	// recomputes it for a closed PR), so a retry of an interrupted batch line
	// must recognise "already landed" before burning the mergeability poll.
	// The same read settles CLOSED-without-merge: any state other than OPEN
	// or MERGED refuses now — polling never makes a closed PR mergeable. A
	// failed read falls through to the poll, which surfaces its own error.
	alreadyMerged := false
	if state, err := prState(num); err == nil {
		switch state {
		case "MERGED":
			alreadyMerged = true
		case "OPEN":
		default:
			return errfmt.NewStructured("land_pr_state_not_merged").
				Set("pr", num).
				Set("state", state)
		}
	}
	if !alreadyMerged {
		// UNKNOWN means GitHub is still recomputing mergeability (async after
		// a sibling PR merges) — poll until resolved. 13/13 batch lands
		// cleared within 4-15s; a separate 5-PR batch needed ~60s (PR #354),
		// so the budget covers the wider window while the 5s interval keeps
		// the common first-retry clear cheap. Only CONFLICTING (or any other
		// non-MERGEABLE, non-UNKNOWN value) is a genuine hard abort.
		const maxAttempts = 20
		type mergeState struct {
			Mergeable        string `json:"mergeable"`
			MergeStateStatus string `json:"mergeStateStatus"`
		}
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
		if worktrees, lerr := gitWorktreeListFn(""); lerr == nil {
			if info, ok := worktrees[headBranch]; ok {
				resolved = info.path
			}
		}
	}
	if resolved == "" && !alreadyMerged {
		// No worktree found via either path: error before merging so the
		// caller can supply --worktree and retry without a half-complete state.
		// (Already-merged retries instead treat this as cleaned below.)
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
	// A PR already MERGED (the retry case above) skips the redundant merge
	// call — gh pr merge on an already-merged PR is itself an error.
	var mergeErr error
	if !alreadyMerged {
		mergeErr = ghPRMergeFn(num)
	}
	finalState, err := prState(num)
	if err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	if finalState != "MERGED" {
		// Surface the merge error when available; fall back to state mismatch.
		if mergeErr != nil {
			// "Base branch was modified" is the transient race — master moved under
			// the PR between the mergeability check and the merge. State is left intact
			// above, so the land is safe to re-run; surface a distinct retryable code,
			// not the generic terminal one (anvil.0140).
			if strings.Contains(mergeErr.Error(), "Base branch was modified") {
				return errfmt.NewStructured("land_pr_base_modified").
					Set("pr", num).
					Set("error", mergeErr.Error()).
					Set("fix_hint", "transient base-modified race; re-run the same --land-pr to retry")
			}
			return errfmt.NewStructured("land_pr_merge_failed").Set("pr", num).Set("error", mergeErr.Error())
		}
		return errfmt.NewStructured("land_pr_state_not_merged").Set("pr", num).Set("state", finalState)
	}
	// Worktree removal stays fatal: a failure here usually means uncommitted
	// work in the worktree, and surfacing it lets the operator recover that work
	// rather than silently discard it. The branches, by contrast, only name a
	// ref whose content is already merged.
	if resolved != "" {
		if err := gitWorktreeRemoveFn(root, resolved); err != nil {
			return errfmt.NewStructured("land_pr_worktree_remove_failed").
				Set("pr", num).
				Set("path", resolved).
				Set("error", err.Error())
		}
	} else {
		// Reachable only when alreadyMerged: the interrupted run removed the
		// worktree before dying, so continue to branch cleanup and resolution.
		fmt.Fprintf(errW, "warning: land-pr %d: worktree not found; assuming the interrupted run already removed it\n", num)
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
