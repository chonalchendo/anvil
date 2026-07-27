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
// transition_side_effects.go (tests swap the vars to stub the gh CLI), plus the
// transition-command policy checks built on those seams — those consume a seam
// and are never swapped.

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

// prState reads a PR's state ("OPEN" / "CLOSED" / "MERGED") through the
// ghPRViewJSON seam. Both --land-pr's post-merge verification and the
// already-resolved gate need it; the shared piece is the read + unmarshal, so
// each caller still attaches its own error code.
//
// gh's own diagnostic ("GraphQL: Could not resolve to a PullRequest with the
// number of 999999") goes to stderr, which .Output() captures into
// *exec.ExitError rather than the error string — an operator handed a bare
// "exit status 1" learns nothing, so the stderr text replaces it here.
func prState(num int) (string, error) {
	raw, err := ghPRViewJSONFn(num, "state")
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
				return "", errors.New(msg)
			}
		}
		return "", err
	}
	var st struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", err
	}
	return st.State, nil
}

// verifyLandPRAlreadyMerged guards the from==to short-circuit in
// newTransitionCmd: `--land-pr` against an already-resolved issue must never
// claim success without reading the PR. It never merges anything (that would
// widen --land-pr's scope beyond an already-resolved issue) — it only
// confirms the PR is genuinely MERGED before letting the short-circuit exit
// 0 (anvil.0214).
//
// Unlike the open-PR resolve check eleven lines away in transition.go, an
// unavailable gh is fatal here rather than a warning: this IS the verification,
// so downgrading it would restore the false success it exists to prevent.
func verifyLandPRAlreadyMerged(id string, prNum int) error {
	state, err := prState(prNum)
	if err != nil {
		e := errfmt.NewStructured("land_pr_view_failed").
			Set("pr", prNum).
			Set("phase", "already_resolved_verify").
			Set("error", err.Error())
		if errors.Is(err, errGhUnavailable) {
			e = e.Set("fix_hint", fmt.Sprintf(
				"cannot verify the PR without gh; re-run when gh is available, or drop --land-pr — `anvil transition issue %s resolved` still exits 0",
				id))
		}
		return e
	}
	if state != "MERGED" {
		return errfmt.NewStructured("land_pr_already_resolved_not_merged").
			Set("pr", prNum).
			Set("state", state).
			Set("fix_hint", "issue is already resolved but the PR is not merged; land the PR, or re-check the issue's status").
			Set("corrected", fmt.Sprintf(
				"anvil transition issue %s open --reason \"<why>\" && anvil transition issue %s resolved --land-pr %d",
				id, id, prNum))
	}
	return nil
}
