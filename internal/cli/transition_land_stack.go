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

type landStackOpts struct {
	t                       core.Type
	id, to                  string
	stack, landPR           int
	override, local, asJSON bool
}

// stackLayer is one PR of the stack and the issue it lands. done marks an issue
// already resolved (skipped); merged marks a PR already MERGED before this run.
type stackLayer struct {
	pr     int
	id     string
	a      *core.Artifact
	ev     landEvidence
	wt     string
	head   landHead
	done   bool
	merged bool
	// mergedHead is the head read after the merge when it differs from head.oid.
	mergedHead string
}

// withLayer names the failing layer's PR on a structured refusal.
func withLayer(err error, pr int) error {
	var se *errfmt.Structured
	if errors.As(err, &se) {
		_ = se.Set("layer_pr", pr)
	}
	return err
}

// headOid re-reads a PR's current head.
func headOid(pr int) (string, error) {
	raw, err := ghPRViewJSONFn(pr, "headRefOid")
	if err != nil {
		return "", err
	}
	var h struct {
		HeadRefOid string `json:"headRefOid"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return "", err
	}
	return h.HeadRefOid, nil
}

// runLandStack gates every layer of the stack topped by opts.stack, then lands
// them all with one gh stack merge. Nothing merges unless every layer passes.
// A re-run after a partial land skips resolved layers and resolves merged ones.
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
	pending := false
	for i := range layers {
		// Layer 0 is based on the default branch; the rest are based on a layer.
		if err := gateLayer(&layers[i], o.local, i > 0); err != nil {
			return printAndReturn(cmd, withLayer(err, layers[i].pr))
		}
		pending = pending || (!layers[i].done && !layers[i].merged)
	}
	for _, l := range layers {
		if l.done || l.merged {
			continue
		}
		if err := landCleanRunFn(errW, landClean{a: l.a, v: v, id: l.id}, root, l.head.oid); err != nil {
			return printAndReturn(cmd, withLayer(err, l.pr))
		}
	}
	if pending {
		if err := mergeLayers(layers, o.stack); err != nil {
			return printAndReturn(cmd, err)
		}
	}
	results := []transitionResult{}
	for _, l := range layers {
		if l.done {
			continue
		}
		r, err := resolveLayer(errW, v, l, root, o.local)
		if err != nil {
			return printAndReturn(cmd, err)
		}
		results = append(results, r)
	}
	if o.asJSON {
		b, _ := json.Marshal(results)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	for _, r := range results {
		if err := emitTransitionJSON(cmd, false, r); err != nil {
			return err
		}
	}
	return nil
}

// mergeLayers runs gh stack merge, which has no head-match flag. Every pending
// head is re-read right before it, so a push or restack since
// the clean run never merges; the head read after it only annotates the audit line.
func mergeLayers(layers []stackLayer, top int) error {
	for _, l := range layers {
		if l.done || l.merged {
			continue
		}
		now, err := headOid(l.pr)
		if err != nil {
			return errfmt.NewStructured("land_pr_view_failed").Set("pr", l.pr).Set("layer_pr", l.pr).Set("error", err.Error())
		}
		if now != l.head.oid {
			return errfmt.NewStructured("land_pr_head_moved").
				Set("pr", l.pr).Set("layer_pr", l.pr).
				Set("verified", l.head.oid).Set("head", now).
				Set("fix_hint", "a layer head moved after the clean run; re-run the same --land-stack to verify the new head")
		}
	}
	mergeErr := ghStackMergeFn(top)
	for i := range layers {
		l := &layers[i]
		if l.done || l.merged {
			continue
		}
		state, err := prState(l.pr)
		switch {
		case err != nil:
			return errfmt.NewStructured("land_stack_state_verify_failed").
				Set("layer_pr", l.pr).Set("error", err.Error()).
				Set("fix_hint", fmt.Sprintf("read PR #%d with gh pr view, then re-run the same --land-stack", l.pr))
		case state == "MERGED":
			// gh stack merge has no head-match flag, so a push in the window can
			// land an unverified head. The merge is done; record the difference.
			if now, err := headOid(l.pr); err != nil {
				l.mergedHead = "unreadable"
			} else if now != l.head.oid {
				l.mergedHead = now
			}
		case mergeErr != nil:
			return errfmt.NewStructured("land_stack_merge_failed").
				Set("layer_pr", l.pr).Set("error", mergeErr.Error()).
				Set("fix_hint", "fix the cause in the error, then re-run the same --land-stack; merged layers are skipped")
		default:
			return errfmt.NewStructured("land_stack_not_merged").
				Set("layer_pr", l.pr).Set("state", state).
				Set("fix_hint", fmt.Sprintf("PR #%d is %s after gh stack merge; check the stack with gh stack view, then re-run the same --land-stack", l.pr, state))
		}
	}
	return nil
}

// walkStack returns the layers bottom-up. The layer below a PR is the PR (any
// state) whose head is that PR's base; the walk ends at a base equal to the
// default branch, and a base with no PR is refused, or gh stack merge could land
// layers that were never gated.
func walkStack(v *core.Vault, topID string, top int) ([]stackLayer, error) {
	def, err := ghDefaultBranchFn()
	if err != nil {
		return nil, errfmt.NewStructured("land_stack_walk_failed").Set("pr", top).Set("error", err.Error()).
			Set("fix_hint", "check gh auth and the remote, then re-run the same --land-stack")
	}
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
		if ref.BaseRefName == def {
			return layers, nil
		}
		url, err := ghPRListFn(ref.BaseRefName, "all")
		if err != nil {
			return nil, errfmt.NewStructured("land_stack_walk_failed").Set("pr", num).Set("error", err.Error()).
				Set("fix_hint", "check gh auth and the remote, then re-run the same --land-stack")
		}
		m := prURLNumber.FindStringSubmatch(url)
		if m == nil {
			return nil, errfmt.NewStructured("land_stack_walk_failed").Set("pr", num).Set("base", ref.BaseRefName).
				Set("fix_hint", fmt.Sprintf("PR #%d is based on %s, which is not the default branch and has no PR; retarget it or land the lower layers with --land-pr, then re-run --land-stack", num, ref.BaseRefName))
		}
		num, _ = strconv.Atoi(m[1])
		id = ""
	}
}

// gateLayer runs the cheap pre-merge gates on one layer and records its head
// and worktree. The clean run is separate so a red cheap gate spares it. A
// resolved issue with a MERGED PR is skipped; a resolved issue whose PR is not
// merged refuses. A MERGED PR skips the review and worktree gates,
// as --land-pr does on a retry.
func gateLayer(l *stackLayer, localValidated, upper bool) error {
	status, _ := l.a.FrontMatter["status"].(string)
	if status == "resolved" {
		state, err := prState(l.pr)
		switch {
		case err != nil:
			return errfmt.NewStructured("land_pr_view_failed").Set("pr", l.pr).Set("error", err.Error())
		case state != "MERGED":
			return errfmt.NewStructured("land_stack_layer_resolved_unmerged").
				Set("issue", l.id).Set("state", state).
				Set("fix_hint", fmt.Sprintf("issue %s is resolved but PR #%d is %s; merge or reopen the PR, or reopen the issue, then re-run --land-stack", l.id, l.pr, state))
		}
		l.done = true
		return nil
	}
	merged, err := landGates(l.pr, localValidated, upper)
	if err != nil {
		return err
	}
	l.merged = merged
	if _, err := core.LookupTransition(core.TypeIssue, status, "resolved"); err != nil {
		return errfmt.NewIllegalTransition(string(core.TypeIssue), l.id, status, "resolved", core.LegalNext(core.TypeIssue, status))
	}
	if l.head, err = l.ev.check(l.pr, l.wt, merged); err != nil {
		return err
	}
	l.wt = resolveLandWorktree(l.wt, l.head.branch)
	if l.wt == "" && !merged {
		return errfmt.NewStructured("land_pr_worktree_missing").Set("pr", l.pr).
			Set("fix_hint", "cut or restore the layer's worktree, then retry")
	}
	return nil
}

// resolveLayer does the post-merge bookkeeping for one merged layer. A worktree
// that will not remove (uncommitted work) is a warning: the PR is already merged.
func resolveLayer(errW io.Writer, v *core.Vault, l stackLayer, root string, local bool) (transitionResult, error) {
	a := l.a
	// A layer merged by an earlier run stamps review_head only from a clean
	// round at its head, as landPR's retry does; a fresh merge was head-checked.
	if rr, ok := latestReviewRound(l.ev.body, l.pr); !l.merged || (ok && !rr.blocked && strings.HasPrefix(l.head.oid, rr.sha)) {
		a.FrontMatter["review_head"] = l.head.oid
	}
	if l.wt != "" {
		if err := gitWorktreeRemoveFn(root, l.wt); err != nil {
			fmt.Fprintf(errW, "warning: land-stack %d: worktree not removed (%s): %v\n", l.pr, l.wt, err)
		}
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
	extra := ""
	if local {
		extra = " (--local-validated: CI gate bypassed on operator attestation)"
	}
	if l.mergedHead != "" {
		extra += fmt.Sprintf(" (merged head %s differs from verified head %s)", l.mergedHead, l.head.oid)
	}
	a.Body += fmt.Sprintf("\n> resolved --land-stack %d %s: merged via gh stack merge + branch deleted%s\n", l.pr, now.Format("2006-01-02"), extra)
	if err := a.Save(); err != nil {
		return transitionResult{}, errfmt.NewStructured("land_pr_succeeded_save_failed").
			Set("issue", l.id).Set("pr", l.pr).Set("error", err.Error()).
			Set("fix_hint", fmt.Sprintf("PR #%d is merged but vault save failed; run `anvil set issue %s status resolved --force --reason \"land-stack merged, vault save failed\"` to repair", l.pr, l.id))
	}
	if err := indexAfterSave(v, a); err != nil {
		return transitionResult{}, err
	}
	return transitionResult{ID: l.id, Path: a.Path, From: from, To: "resolved", Status: "transitioned"}, nil
}

// ghStackMergeReal merges the whole stack topped by PR num in one call.
// gh stack merge has no head-match flag, so the caller re-reads every head
// right before this call.
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

// ghDefaultBranchReal names the repo's default branch.
func ghDefaultBranchReal() (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errGhUnavailable
	}
	out, err := exec.Command("gh", "repo", "view", "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name").Output() //nolint:gosec // binary path resolved from trusted sources; not user input
	if err != nil {
		return "", fmt.Errorf("gh repo view: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
