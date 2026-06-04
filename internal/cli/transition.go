package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

func newTransitionCmd() *cobra.Command {
	var owner, reason string
	var asJSON, force, noLongerReproduces, cutWorktree bool
	var worktreeOverride, branchOverride string
	var landPRNum int
	cmd := &cobra.Command{
		Use:   "transition <type> <id> <new-state>",
		Short: "Move an artifact through its state machine",
		Args:  namedArgs("anvil transition <type> <id> <new-state>", []string{"<type>", "<id>", "<new-state>"}, 3, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return fmt.Errorf("type: %w", err)
			}
			id, to := args[1], args[2]

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			// Resolve bare ordinal ("0042") and project-qualified ordinal
			// ("anvil.0042") to the full ID, matching the read path in show.go.
			if t == core.TypeIssue {
				if proj, ord, ok := core.ParseProjectQualifiedOrdinal(id); ok {
					if resolved, ok := core.ResolveIssueOrdinal(v, proj, ord); ok {
						id = resolved
					}
				} else if core.IsOrdinalOnly(id) {
					if p, err := core.ResolveProject(); err == nil {
						if resolved, ok := core.ResolveIssueOrdinal(v, p.Slug, id); ok {
							id = resolved
						}
					}
				}
			}

			path := filepath.Join(v.Root, t.Dir(), id+".md")
			a, err := core.LoadArtifact(path)
			if err != nil {
				return ErrArtifactNotFound
			}

			from, _ := a.FrontMatter["status"].(string)
			if from == to {
				// A re-claim of an already-in-progress issue lands here. Same
				// session (or no session id on either side) is idempotent; a
				// different session under the same owner is refused unless
				// --force, so two parallel sessions can't both adopt the claim.
				if t == core.TypeIssue && to == "in-progress" {
					if !force {
						if err := claimConflict(a, id, os.Getenv(envSessionID)); err != nil {
							return printAndReturn(cmd, err)
						}
					} else if sid := os.Getenv(envSessionID); sid != "" {
						// --force takeover: transfer the claim to the current session
						// so the "take over the claim" hint is truthful and the new
						// session can subsequently re-claim idempotently.
						a.FrontMatter["claim_session"] = sid
						a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
						if err := a.Save(); err != nil {
							return fmt.Errorf("saving claim takeover: %w", err)
						}
						if err := indexAfterSave(v, a); err != nil {
							return err
						}
					}
				}
				return emitTransitionJSON(cmd, asJSON, transitionResult{
					ID: id, Path: path, From: from, To: to, Status: "already_in_state",
				})
			}

			if cutWorktree && (t != core.TypeIssue || to != "in-progress") {
				return printAndReturn(cmd, errfmt.NewStructured("invalid_flag_for_transition").
					Set("flag", "--cut-worktree").
					Set("applies_to", "transition issue <id> in-progress"))
			}
			if branchOverride != "" && !cutWorktree {
				return printAndReturn(cmd, errfmt.NewStructured("invalid_flag_for_transition").
					Set("flag", "--branch").
					Set("applies_to", "use only with --cut-worktree"))
			}
			if worktreeOverride != "" && !cutWorktree && landPRNum == 0 {
				return printAndReturn(cmd, errfmt.NewStructured("invalid_flag_for_transition").
					Set("flag", "--worktree").
					Set("applies_to", "use only with --cut-worktree or --land-pr"))
			}
			if landPRNum != 0 {
				if t != core.TypeIssue || to != "resolved" {
					return printAndReturn(cmd, errfmt.NewStructured("invalid_flag_for_transition").
						Set("flag", "--land-pr").
						Set("applies_to", "transition issue <id> resolved"))
				}
				if force || noLongerReproduces {
					return printAndReturn(cmd, errfmt.NewStructured("flags_conflict").
						Set("flags", []string{"--land-pr", "--force/--no-longer-reproduces"}).
						Set("fix_hint", "--land-pr merges and verifies; do not combine with override flags"))
				}
			}

			if noLongerReproduces {
				if force {
					return printAndReturn(cmd, errfmt.NewStructured("flags_conflict").
						Set("flags", []string{"--force", "--no-longer-reproduces"}).
						Set("fix_hint", "use one or the other"))
				}
				if t != core.TypeIssue || to != "in-progress" {
					return printAndReturn(cmd, errfmt.NewStructured("invalid_flag_for_transition").
						Set("flag", "--no-longer-reproduces").
						Set("applies_to", "transition issue <id> in-progress"))
				}
				if from != "open" {
					return printAndReturn(cmd, errfmt.NewIllegalTransition(string(t), id, from, "in-progress", core.LegalNext(t, from)))
				}
				ok, anchorCmd, diff, aerr := runAnchorCheck(cmd.Context(), a, cmd.ErrOrStderr())
				if aerr != nil {
					return fmt.Errorf("anchor check: %w", aerr)
				}
				if anchorCmd == "" {
					return printAndReturn(cmd, errfmt.NewStructured("no_anchor_to_check").
						Set("issue", id).
						Set("fix_hint", "--no-longer-reproduces requires a reproduction_anchor on the issue"))
				}
				if ok {
					return printAndReturn(cmd, errfmt.NewStructured("anchor_still_reproduces").
						Set("issue", id).
						Set("command", anchorCmd).
						Set("fix_hint", "issue still reproduces; claim and fix, do not close as stale"))
				}
				stamp := time.Now().UTC().Format("2006-01-02")
				audit := fmt.Sprintf("\n> resolved --no-longer-reproduces %s: anchor no longer reproduces:\n%s\n", stamp, diff)
				if !strings.HasSuffix(a.Body, "\n") {
					a.Body += "\n"
				}
				a.Body += audit
				a.FrontMatter["status"] = "resolved"
				a.FrontMatter["updated"] = stamp
				if err := a.Save(); err != nil {
					return fmt.Errorf("saving: %w", err)
				}
				if err := indexAfterSave(v, a); err != nil {
					return err
				}
				return emitTransitionJSON(cmd, asJSON, transitionResult{
					ID: id, Path: path, From: from, To: "resolved", Status: "transitioned",
				})
			}

			tr, err := core.LookupTransition(t, from, to)
			if err != nil {
				e := errfmt.NewIllegalTransition(string(t), id, from, to, core.LegalNext(t, from))
				return printAndReturn(cmd, e)
			}

			flagValues := map[string]string{"owner": owner, "reason": reason}
			for _, flag := range tr.Requires {
				if flagValues[flag] == "" {
					return printAndReturn(cmd, errfmt.NewTransitionFlagRequired(string(t), id, from, to, flag))
				}
			}
			if tr.Reverse && reason == "" {
				return printAndReturn(cmd, errfmt.NewTransitionFlagRequired(string(t), id, from, to, "reason"))
			}

			if t == core.TypePlan && to == "locked" {
				p, lerr := core.LoadPlan(path)
				if lerr != nil {
					return fmt.Errorf("plan validator: %w", lerr)
				}
				if verr := core.ValidatePlan(p); verr != nil {
					return fmt.Errorf("plan validator: %w", verr)
				}
			}

			// Backfill-on-claim: refuse issue → in-progress unless goal: is set.
			// The back-catalogue predates the field; this lazily forces a
			// one-sentence terminal predicate at claim time rather than via a
			// sweep. --force is the escape hatch, matching the anchor gate below.
			if t == core.TypeIssue && to == "in-progress" && !force {
				if goal, _ := a.FrontMatter["goal"].(string); strings.TrimSpace(goal) == "" {
					e := errfmt.NewStructured("missing_goal").
						Set("issue", id).
						Set("fix_hint", "set a one-sentence goal before claiming: anvil set issue "+id+` goal "<what done means>"`)
					return printAndReturn(cmd, e)
				}
			}

			// Refuse issue → in-progress when the recorded reproduction_anchor
			// no longer matches observed output. Grandfathers issues that have
			// no anchor.
			if t == core.TypeIssue && to == "in-progress" && !force {
				ok, anchorCmd, diff, aerr := runAnchorCheck(cmd.Context(), a, cmd.ErrOrStderr())
				if aerr != nil {
					return fmt.Errorf("anchor check: %w", aerr)
				}
				if !ok {
					e := errfmt.NewStructured("anchor_mismatch").
						Set("issue", id).
						Set("command", anchorCmd).
						Set("diff", diff).
						Set("fix_hint", "rerun with --force to claim anyway, or --no-longer-reproduces to close as stale")
					return printAndReturn(cmd, e)
				}
			}

			if cutWorktree {
				if err := doCutWorktree(a, id, worktreeOverride, branchOverride); err != nil {
					return printAndReturn(cmd, err)
				}
			}

			if landPRNum != 0 {
				if err := doLandPR(a, id, landPRNum, worktreeOverride); err != nil {
					return printAndReturn(cmd, err)
				}
			}

			// Refuse issue → resolved when the issue's anvil/<slug> branch
			// still has an open PR, unless --force is set. Uniform across
			// every codepath that calls `anvil transition`. See
			// transition_pr_check.go for branch-candidate resolution.
			if t == core.TypeIssue && to == "resolved" && !force {
				branch, prURL, warn, qerr := openPRForIssueResolve(v, id)
				if warn != "" {
					cmd.PrintErrln("warning: " + warn)
				}
				switch {
				case errors.Is(qerr, errGhUnavailable):
					cmd.PrintErrln("warning: gh unavailable; skipping open-PR refusal check")
				case qerr != nil:
					return fmt.Errorf("checking for open PR: %w", qerr)
				case prURL != "":
					return printAndReturn(cmd, errfmt.NewOpenPRBlocksResolve(id, branch, prURL))
				}
			}

			a.FrontMatter["status"] = to
			if owner != "" {
				a.FrontMatter["owner"] = owner
			}
			// Stamp the claiming session so a later same-owner claim from a
			// different session can be refused. Omitted outside a Claude session
			// (env unset) — there is no identity to record.
			if t == core.TypeIssue && to == "in-progress" {
				if sid := os.Getenv(envSessionID); sid != "" {
					a.FrontMatter["claim_session"] = sid
				}
			}
			a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")

			if tr.Reverse {
				stamp := time.Now().UTC().Format("2006-01-02")
				audit := fmt.Sprintf("\n> reopened %s: %s\n", stamp, reason)
				if !strings.HasSuffix(a.Body, "\n") {
					a.Body += "\n"
				}
				a.Body += audit
			}

			if t == core.TypeIssue && to == "resolved" && force {
				stamp := time.Now().UTC().Format("2006-01-02")
				note := reason
				if note == "" {
					note = "no reason given"
				}
				audit := fmt.Sprintf("\n> resolved --force %s: %s\n", stamp, note)
				if !strings.HasSuffix(a.Body, "\n") {
					a.Body += "\n"
				}
				a.Body += audit
			}

			if t == core.TypeIssue && to == "resolved" && landPRNum != 0 {
				stamp := time.Now().UTC().Format("2006-01-02")
				audit := fmt.Sprintf("\n> resolved --land-pr %d %s: merged via squash + branch deleted\n", landPRNum, stamp)
				if !strings.HasSuffix(a.Body, "\n") {
					a.Body += "\n"
				}
				a.Body += audit
			}

			if err := a.Save(); err != nil {
				if t == core.TypeIssue && to == "resolved" && landPRNum != 0 {
					return printAndReturn(cmd, errfmt.NewStructured("land_pr_succeeded_save_failed").
						Set("issue", id).
						Set("pr", landPRNum).
						Set("error", err.Error()).
						Set("fix_hint", fmt.Sprintf("PR #%d is merged but vault save failed; run `anvil set issue %s status resolved` to repair", landPRNum, id)))
				}
				return fmt.Errorf("saving: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return err
			}

			return emitTransitionJSON(cmd, asJSON, transitionResult{
				ID: id, Path: path, From: from, To: to, Owner: owner, Reason: reason, Status: "transitioned",
			})
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "owner (required for claim transitions)")
	cmd.Flags().StringVar(&reason, "reason", "", "audit reason (required for reverse transitions)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON envelope")
	cmd.Flags().BoolVar(&force, "force", false, "override the open-PR refusal on issue → resolved (audit-logged)")
	cmd.Flags().BoolVar(&noLongerReproduces, "no-longer-reproduces", false, "on a mismatching reproduction_anchor, close the issue as resolved with the diff captured (mutually exclusive with --force)")
	cmd.Flags().BoolVar(&cutWorktree, "cut-worktree", false, "create the conventional worktree+branch before transitioning (issue → in-progress only)")
	cmd.Flags().StringVar(&worktreeOverride, "worktree", "", "override the derived worktree path (used with --cut-worktree or --land-pr)")
	cmd.Flags().StringVar(&branchOverride, "branch", "", "override the derived branch name (used with --cut-worktree)")
	cmd.Flags().IntVar(&landPRNum, "land-pr", 0, "PR number to land: verify-mergeable + CI-green, remove worktree, squash-merge and delete branch, verify MERGED, then transition (issue → resolved only)")
	return cmd
}

type transitionResult struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	From   string `json:"from"`
	To     string `json:"to"`
	Owner  string `json:"owner,omitempty"`
	Reason string `json:"reason,omitempty"`
	Status string `json:"status"`
}

func emitTransitionJSON(cmd *cobra.Command, asJSON bool, r transitionResult) error {
	if asJSON {
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	if r.Status == "already_in_state" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s already in state %s\n", r.ID, r.To)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", r.ID, r.From, r.To)
	return nil
}

// printAndReturn honors --json like the success path: when set, the JSON
// envelope goes to stdout and we return nil so fang doesn't also render the
// human block; when unset, we return the error and let fang render it.
// GetBool returns false for callers that never registered --json, so the
// human-only path is preserved for list/plan/tags callers.
func printAndReturn(cmd *cobra.Command, err error) error {
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		b, _ := json.Marshal(err)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	return err
}
