package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/chonalchendo/anvil/internal/core"
)

// EnsurePRForTask is the engine's production advance-gate: it ensures an open PR
// exists for the deterministic branch the driver cut for the task (t.Branch),
// landing the spawn's verified diff itself when the spawn left it unlanded. The
// driver wires it into Options.VerifyArtifact — for the complete phase only, so
// review and respond never land — and a task that still has no PR afterwards is
// recorded "failed", not success (anvil.0112).
//
// Landing is the harness's job, not the agent's (anvil.0162): a headless worker
// that exits 0 with real edits sitting uncommitted used to be recorded "opened
// no PR" and its work thrown away. So the gate reads: PR already open → success;
// otherwise commit + push + open the PR from the worktree, then re-check. It
// only ever runs on a clean exit 0, so the tree it lands is one the worker
// verified.
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
func EnsurePRForTask(ctx context.Context, t core.Task) (bool, error) {
	if t.Branch == "" {
		return false, fmt.Errorf("task %s has no branch to check for a PR", t.ID)
	}
	// Cwd is optional to the engine — it falls back to Options.Cwd — but not
	// here: every git command below would run with cmd.Dir unset, i.e. in the
	// driver's own checkout on master. CLAUDE.md forbids committing there.
	if t.Cwd == "" {
		return false, fmt.Errorf("task %s has no worktree to land from", t.ID)
	}
	open, err := openPRExists(ctx, t)
	if err != nil || open {
		return open, err
	}
	url, err := landTaskDiff(ctx, t)
	if err != nil {
		if errors.Is(err, errNothingToLand) {
			return false, nil
		}
		return false, fmt.Errorf("landing task %s: %w", t.ID, err)
	}
	slog.Info("build: driver landed task", "task", t.ID, "branch", t.Branch, "pr", url)
	return openPRExists(ctx, t)
}

func openPRExists(ctx context.Context, t core.Task) (bool, error) {
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
