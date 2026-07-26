package build

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// errNothingToLand reports that the branch carries no diff at all — nothing was
// committed and nothing sits ahead of the base. Landing is not the answer there;
// the spawn produced no work (the retry/nudge path, anvil.0163), so the
// advance-gate keeps failing the task.
var errNothingToLand = errors.New("branch has no diff to land")

// landTaskDiff stages, commits, pushes, and opens the PR for whatever the
// complete spawn left on the task's branch.
//
// The inversion is the point (anvil.0162): a headless `claude -p` worker that
// self-judges "implement X" done after editing files routinely never reaches its
// own commit, so the work and its landing diverge and an exit-0 spawn with real
// edits is recorded "opened no PR". Landing is a deterministic harness step, not
// an agent side-effect the prompt hopes for.
//
// Only reachable from the complete-phase advance-gate after a clean exit 0, so
// the tree committed here is one the worker ran the issue's `## Verification`
// and the build gate against — the driver never commits an unverified tree. The
// review and respond phases wire no gate, so they never land.
func landTaskDiff(ctx context.Context, t core.Task) error {
	if _, err := git(ctx, t.Cwd, "add", "-A"); err != nil {
		return err
	}
	staged, err := git(ctx, t.Cwd, "diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	if strings.TrimSpace(staged) != "" {
		if _, err := git(ctx, t.Cwd, "commit", "-m", commitMessage(t)); err != nil {
			return err
		}
	}
	// A worker that committed but never pushed or opened its PR lands here too:
	// nothing stages, but the branch is ahead of the base it was cut from.
	ahead, err := git(ctx, t.Cwd, "rev-list", "--count", "origin/HEAD..HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(ahead) == "0" {
		return errNothingToLand
	}
	if _, err := git(ctx, t.Cwd, "push", "--set-upstream", "origin", t.Branch); err != nil {
		return err
	}
	label := t.Title
	if label == "" {
		label = t.ID
	}
	body := fmt.Sprintf("Landed by `anvil build` from the verified worktree for %s.\n\nResolves %s\n", t.ID, t.ID)
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", //nolint:gosec // branch/id are package-controlled slugs, never user input
		"--head", t.Branch, "--title", "build: "+label, "--body", body)
	cmd.Dir = t.Cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh pr create --head %s: %w: %s", t.Branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// commitMessage labels the driver-landed commit. The PR squash-merges, so this
// is an audit line — it names the task whose verified tree it carries.
func commitMessage(t core.Task) string {
	return fmt.Sprintf("build: land verified diff for %s", t.ID)
}

// git runs one git command in dir and returns its stdout. Errors carry stderr so
// a failed landing is diagnosable from the task's Diagnostic alone.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // args are package-controlled, never user input
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
