package cli

import (
	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// cutWorktreeFlagRefusal refuses the cut-worktree flag family on a transition
// it does not apply to. It runs before the already-in-state short-circuit so a
// misplaced flag is never silently ignored.
func cutWorktreeFlagRefusal(t core.Type, to string, cutWorktree bool, worktree, branch, base string, landPR int) error {
	if cutWorktree && (t != core.TypeIssue || to != "in-progress") {
		return errfmt.NewStructured("invalid_flag_for_transition").
			Set("flag", "--cut-worktree").
			Set("applies_to", "transition issue <id> in-progress")
	}
	if branch != "" && !cutWorktree {
		return errfmt.NewStructured("invalid_flag_for_transition").
			Set("flag", "--branch").
			Set("applies_to", "use only with --cut-worktree")
	}
	if base != "" && !cutWorktree {
		return errfmt.NewStructured("invalid_flag_for_transition").
			Set("flag", "--base").
			Set("applies_to", "use only with --cut-worktree")
	}
	if worktree != "" && !cutWorktree && landPR == 0 {
		return errfmt.NewStructured("invalid_flag_for_transition").
			Set("flag", "--worktree").
			Set("applies_to", "use only with --cut-worktree or --land-pr")
	}
	return nil
}
