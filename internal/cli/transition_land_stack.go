package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// landGates is the pre-merge PR gate --land-pr and --land-stack share: state,
// mergeability, then required CI. It reports a PR that is already MERGED.
func landGates(num int, localValidated bool) (alreadyMerged bool, err error) {
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
			if err := ghPRChecksFn(num); err != nil {
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

// ghStackMergeReal merges the whole stack topped by PR num in one call.
func ghStackMergeReal(num int) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	out, err := exec.Command("gh", "stack", "merge", strconv.Itoa(num), "--squash", "--yes").CombinedOutput() //nolint:gosec // binary path resolved from trusted sources; not user input
	if err != nil {
		return fmt.Errorf("gh stack merge: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type landStackOpts struct {
	t                       core.Type
	id, to                  string
	stack, landPR           int
	override, local, asJSON bool
}

// stackLayer is one PR of the stack and the issue it lands.
type stackLayer struct {
	pr   int
	id   string
	a    *core.Artifact
	ev   landEvidence
	wt   string
	head landHead
}

// withLayer names the failing layer's PR on a structured refusal.
func withLayer(err error, pr int) error {
	var se *errfmt.Structured
	if errors.As(err, &se) {
		se.Set("layer_pr", pr)
	}
	return err
}

// runLandStack gates every layer of the stack topped by opts.stack, then lands
// them all with one gh stack merge. Nothing merges unless every layer passes.
func runLandStack(cmd *cobra.Command, o landStackOpts) error {
	errW := cmd.ErrOrStderr()
	switch {
	case o.landPR != 0:
		return printAndReturn(cmd, errfmt.NewStructured("land_flags_exclusive").
			Set("flags", []string{"--land-stack", "--land-pr"}).
			Set("fix_hint", "--land-stack lands every layer of the stack; --land-pr lands one PR"))
	case o.t != core.TypeIssue || o.to != "resolved":
		return printAndReturn(cmd, errfmt.NewStructured("invalid_flag_for_transition").
			Set("flag", "--land-stack").
			Set("applies_to", "transition issue <id> resolved"))
	case o.override:
		return printAndReturn(cmd, errfmt.NewStructured("flags_conflict").
			Set("flags", []string{"--land-stack", "--force/--no-longer-reproduces"}).
			Set("fix_hint", "--land-stack merges and verifies; do not combine with override flags"))
	}
	v, err := core.ResolveVault()
	if err != nil {
		return fmt.Errorf("resolving vault: %w", err)
	}
	layers, err := walkStack(v, o.id, o.stack)
	if err != nil {
		return printAndReturn(cmd, err)
	}
	root, err := gitMainRootFn()
	if err != nil {
		return printAndReturn(cmd, errfmt.NewStructured("land_pr_main_worktree_lookup_failed").Set("error", err.Error()))
	}
	// The run is usually fired from inside a layer worktree that is removed below.
	if err := os.Chdir(root); err != nil {
		return printAndReturn(cmd, errfmt.NewStructured("land_pr_chdir_root_failed").Set("root", root).Set("error", err.Error()))
	}
	for i := range layers {
		if err := gateLayer(&layers[i], o.local); err != nil {
			return printAndReturn(cmd, withLayer(err, layers[i].pr))
		}
	}
	for _, l := range layers {
		if err := landCleanRunFn(errW, landClean{a: l.a, v: v, id: l.id}, root, l.head.oid); err != nil {
			return printAndReturn(cmd, withLayer(err, l.pr))
		}
	}
	mergeErr := ghStackMergeFn(o.stack)
	for _, l := range layers {
		state, err := prState(l.pr)
		if err == nil && state == "MERGED" {
			continue
		}
		if mergeErr != nil {
			return printAndReturn(cmd, errfmt.NewStructured("land_stack_merge_failed").
				Set("layer_pr", l.pr).Set("error", mergeErr.Error()))
		}
		return printAndReturn(cmd, errfmt.NewStructured("land_stack_not_merged").
			Set("layer_pr", l.pr).Set("state", state))
	}
	for _, l := range layers {
		if err := resolveLayer(cmd, v, l, root, o.asJSON); err != nil {
			return printAndReturn(cmd, err)
		}
	}
	return nil
}

// walkStack returns the layers bottom-up. The layer below a PR is the open PR
// whose head is that PR's base; no such PR means the base is the default branch.
func walkStack(v *core.Vault, topID string, top int) ([]stackLayer, error) {
	var layers []stackLayer
	for num, id := top, topID; ; {
		raw, err := ghPRViewJSONFn(num, "baseRefName,headRefName")
		if err != nil {
			return nil, errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
		}
		var ref struct{ BaseRefName, HeadRefName string }
		if err := json.Unmarshal(raw, &ref); err != nil {
			return nil, errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
		}
		if id == "" {
			// Branch <project>/<ordinal>.<slug> names issue <project>.<ordinal>.<slug>.
			id = strings.Replace(ref.HeadRefName, "/", ".", 1)
		}
		cid, path, err := core.ResolveArtifact(v, core.TypeIssue, id)
		if err != nil {
			return nil, withLayer(err, num)
		}
		a, err := core.LoadArtifact(path)
		if err != nil {
			return nil, withLayer(err, num)
		}
		project, slug := projectFromArtifact(a, cid), slugFromIssueID(cid)
		wt, err := defaultWorktreePath(project, slug)
		if err != nil {
			return nil, errfmt.NewStructured("land_pr_path_failed").Set("error", err.Error())
		}
		layers = append([]stackLayer{{pr: num, id: cid, a: a, ev: newLandEvidence(a, cid, project, slug), wt: wt}}, layers...)
		url, err := ghPRListFn(ref.BaseRefName)
		if err != nil {
			return nil, errfmt.NewStructured("land_stack_walk_failed").Set("pr", num).Set("error", err.Error())
		}
		m := prURLNumber.FindStringSubmatch(url)
		if m == nil {
			return layers, nil
		}
		num, _ = strconv.Atoi(m[1])
		id = ""
	}
}

// gateLayer runs the cheap pre-merge gates on one layer and records its head
// and worktree. The clean run is separate so a red cheap gate spares it.
func gateLayer(l *stackLayer, localValidated bool) error {
	merged, err := landGates(l.pr, localValidated)
	if err != nil {
		return err
	}
	if merged {
		return errfmt.NewStructured("land_stack_layer_merged").Set("pr", l.pr)
	}
	status, _ := l.a.FrontMatter["status"].(string)
	if _, err := core.LookupTransition(core.TypeIssue, status, "resolved"); err != nil {
		return errfmt.NewIllegalTransition(string(core.TypeIssue), l.id, status, "resolved", core.LegalNext(core.TypeIssue, status))
	}
	if l.head, err = l.ev.check(l.pr, l.wt, false); err != nil {
		return err
	}
	if l.wt = resolveLandWorktree(l.wt, l.head.branch); l.wt == "" {
		return errfmt.NewStructured("land_pr_worktree_missing").Set("pr", l.pr).
			Set("fix_hint", "cut or restore the layer's worktree, then retry")
	}
	return nil
}

// resolveLayer does the post-merge bookkeeping for one merged layer. A worktree
// that will not remove (uncommitted work) is a warning: the PR is already merged.
func resolveLayer(cmd *cobra.Command, v *core.Vault, l stackLayer, root string, asJSON bool) error {
	errW := cmd.ErrOrStderr()
	a := l.a
	a.FrontMatter["review_head"] = l.head.oid
	if err := gitWorktreeRemoveFn(root, l.wt); err != nil {
		fmt.Fprintf(errW, "warning: land-stack %d: worktree not removed (%s): %v\n", l.pr, l.wt, err)
	}
	deleteLandBranches(errW, l.pr, root, l.head.branch)
	stampLandCost(errW, a, l.id, l.pr)
	a.Body += postLandRedNote(runPostLandBlocks(errW, a.Body), a.Body)
	from, _ := a.FrontMatter["status"].(string)
	now := time.Now().UTC()
	a.FrontMatter["status"] = "resolved"
	stampIssueGate(a, "resolved", "", now)
	a.FrontMatter["updated"] = now.Format("2006-01-02")
	if !strings.HasSuffix(a.Body, "\n") {
		a.Body += "\n"
	}
	a.Body += fmt.Sprintf("\n> resolved --land-stack %d %s: merged via gh stack merge + branch deleted\n", l.pr, now.Format("2006-01-02"))
	if err := a.Save(); err != nil {
		return errfmt.NewStructured("land_pr_succeeded_save_failed").
			Set("issue", l.id).Set("pr", l.pr).Set("error", err.Error()).
			Set("fix_hint", fmt.Sprintf("PR #%d is merged but vault save failed; run `anvil set issue %s status resolved --force --reason \"land-stack merged, vault save failed\"` to repair", l.pr, l.id))
	}
	if err := indexAfterSave(v, a); err != nil {
		return err
	}
	return emitTransitionJSON(cmd, asJSON, transitionResult{ID: l.id, Path: a.Path, From: from, To: "resolved", Status: "transitioned"})
}
