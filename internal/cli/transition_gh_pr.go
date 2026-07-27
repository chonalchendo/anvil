package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

// Real implementations behind the ghPR* package-level seams declared in
// transition_side_effects.go; tests swap the vars to stub the gh CLI.

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

// verifyLandPRAlreadyMerged guards the from==to short-circuit in
// newTransitionCmd: `--land-pr` against an already-resolved issue must never
// claim success without reading the PR. It never merges anything (that would
// widen --land-pr's scope beyond an already-resolved issue) — it only
// confirms the PR is genuinely MERGED before letting the short-circuit exit
// 0 (anvil.0214).
func verifyLandPRAlreadyMerged(prNum int) error {
	raw, err := ghPRViewJSONFn(prNum, "state")
	if err != nil {
		return errfmt.NewStructured("land_pr_view_failed").Set("pr", prNum).Set("error", err.Error())
	}
	var st struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return errfmt.NewStructured("land_pr_view_failed").Set("pr", prNum).Set("error", err.Error())
	}
	if st.State != "MERGED" {
		return errfmt.NewStructured("land_pr_already_resolved_not_merged").
			Set("pr", prNum).
			Set("state", st.State).
			Set("fix_hint", "issue is already resolved but the PR is not merged; land the PR, or re-check the issue's status")
	}
	return nil
}
