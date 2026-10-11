package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

// landGates is the pre-merge PR gate --land-pr and --land-stack share: state,
// mergeability, then CI. It reports a PR that is already MERGED. allChecks gates
// on every reported check, not only required ones: branch protection guards the
// default branch alone, so a stack layer based on another layer has none.
func landGates(num int, localValidated, allChecks bool) (alreadyMerged bool, err error) {
	// An already-MERGED PR also reads mergeable:UNKNOWN (GitHub never
	// recomputes it for a closed PR), so a retry of an interrupted batch line
	// must recognise "already landed" before burning the mergeability poll.
	// The same read settles CLOSED-without-merge: any state other than OPEN
	// or MERGED refuses now — polling never makes a closed PR mergeable. A
	// failed read falls through to the poll, which surfaces its own error.
	if state, err := prState(num); err == nil {
		switch state {
		case "MERGED":
			alreadyMerged = true
		case "OPEN":
		default:
			return false, errfmt.NewStructured("land_pr_state_not_merged").
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
				return false, errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
			}
			if err := json.Unmarshal(raw, &st); err != nil {
				return false, errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
			}
			if st.Mergeable != "UNKNOWN" {
				break
			}
			if attempt < maxAttempts-1 {
				mergeabilityPollSleep(5 * time.Second)
			}
		}
		if st.Mergeable != "MERGEABLE" {
			return false, errfmt.NewStructured("land_pr_not_mergeable").
				Set("pr", num).
				Set("mergeable", st.Mergeable).
				Set("merge_state", st.MergeStateStatus)
		}
		if !localValidated {
			if err := ghPRChecksFn(num, !allChecks); err != nil {
				return false, errfmt.NewStructured("land_pr_ci_not_green").Set("pr", num).Set("error", err.Error())
			}
		}
	}
	return alreadyMerged, nil
}

// resolveLandWorktree returns the explicit path when it exists, else the live
// worktree checked out on the PR's head branch, else "".
func resolveLandWorktree(worktreePath, headBranch string) string {
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
	return resolved
}

// deleteLandBranches removes the merged PR's local then remote branch.
func deleteLandBranches(errW io.Writer, num int, root, headBranch string) {
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
}
