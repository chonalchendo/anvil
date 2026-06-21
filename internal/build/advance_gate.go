package build

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// PRExistsForTask is the engine's production advance-gate: it reports whether a
// PR exists for the branch checked out in the task's worktree (t.Cwd) — the
// deterministic <project>/<slug> branch the driver cut before dispatch. The
// driver wires it into Options.VerifyArtifact so a spawn that exits 0 without
// opening a PR is recorded "failed", not success (anvil.0112).
//
// It runs `gh pr view` with the worktree as the working directory so gh resolves
// the PR for that worktree's current branch — the key the driver already holds,
// not a slug the engine recomputes. The command inherits the parent process env;
// it must never run under the spawn's isolated CLAUDE_CONFIG_DIR, which strips gh
// auth on Keychain-backed macOS.
func PRExistsForTask(ctx context.Context, t core.Task) (bool, error) {
	if t.Cwd == "" {
		return false, fmt.Errorf("task %s has no worktree to check for a PR", t.ID)
	}
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "state", "--jq", ".state")
	cmd.Dir = t.Cwd
	out, err := cmd.Output()
	if err != nil {
		// gh exits non-zero with "no pull requests found" when the branch has no
		// PR — that is the no-artifact case the gate exists to catch, not an infra
		// failure. Any other non-zero (auth, gh missing) is a real error so the
		// gate fails loud rather than masking it as a no-op success.
		var ee *exec.ExitError
		if errors.As(err, &ee) && strings.Contains(strings.ToLower(string(ee.Stderr)), "no pull requests found") {
			return false, nil
		}
		return false, fmt.Errorf("gh pr view in %s: %w", t.Cwd, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}
