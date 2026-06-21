package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/chonalchendo/anvil/internal/core"
)

// PRExistsForTask is the engine's production advance-gate: it reports whether an
// open PR exists for the deterministic branch the driver cut for the task
// (t.Branch). The driver wires it into Options.VerifyArtifact so a spawn that
// exits 0 without opening a PR is recorded "failed", not success (anvil.0112).
//
// It runs `gh pr list --head t.Branch --state open` so it checks the exact
// branch the driver holds — not whatever the worktree HEAD points at — and
// counts only an open PR the review phase can act on (a closed or merged PR is
// not the advance artifact). gh returns an empty array with exit 0 when no PR
// matches, so a non-zero exit is a genuine failure (gh missing, auth, network)
// and the gate fails loud rather than masking an unverifiable artifact as
// success. The command inherits the parent env; it must never run under the
// spawn's isolated CLAUDE_CONFIG_DIR, which strips gh auth on Keychain-backed
// macOS.
func PRExistsForTask(ctx context.Context, t core.Task) (bool, error) {
	if t.Branch == "" {
		return false, fmt.Errorf("task %s has no branch to check for a PR", t.ID)
	}
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", //nolint:gosec // branch is a package-controlled <project>/<slug> slug, never user input
		"--head", t.Branch, "--state", "open", "--json", "number", "--limit", "1")
	cmd.Dir = t.Cwd
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("gh pr list --head %s: %w", t.Branch, err)
	}
	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return false, fmt.Errorf("gh pr list: parse: %w", err)
	}
	return len(prs) > 0, nil
}
