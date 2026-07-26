package build

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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
// It returns the url `gh pr create` printed, so the caller can stamp the PR onto
// the issue the worker no longer stamps itself.
func landTaskDiff(ctx context.Context, t core.Task) (string, error) {
	// Everything below measures, commits and pushes HEAD, but the PR is opened on
	// t.Branch. A spawn that switched or detached HEAD would otherwise produce a
	// PR that does not contain the work the gate just measured — a false success.
	head, err := git(ctx, t.Cwd, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("task %s: HEAD is not on a branch: %w", t.ID, err)
	}
	if strings.TrimSpace(head) != t.Branch {
		return "", fmt.Errorf("task %s: HEAD is on %q, not the driver's branch %q", t.ID, strings.TrimSpace(head), t.Branch)
	}
	if _, err := git(ctx, t.Cwd, "add", "-A"); err != nil {
		return "", err
	}
	staged, err := git(ctx, t.Cwd, "diff", "--cached", "--name-only")
	if err != nil {
		return "", err
	}
	if path := firstNeverCommit(staged); path != "" {
		return "", fmt.Errorf("task %s: refusing to land — staged path %q is never committed (docs/git-conventions.md)", t.ID, path)
	}
	if strings.TrimSpace(staged) != "" {
		if _, err := git(ctx, t.Cwd, "commit", "-m", commitMessage(t)); err != nil {
			return "", err
		}
	}
	// A worker that committed but never pushed or opened its PR lands here too:
	// nothing stages, but the branch is ahead of the base it was cut from.
	ahead, err := git(ctx, t.Cwd, "rev-list", "--count", "origin/HEAD..HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ahead) == "0" {
		return "", errNothingToLand
	}
	if _, err := git(ctx, t.Cwd, "push", "--set-upstream", "origin", t.Branch); err != nil {
		return "", err
	}
	label := t.Title
	if label == "" {
		label = t.ID
	}
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", //nolint:gosec // branch/id are package-controlled slugs, never user input
		"--head", t.Branch, "--title", "chore: land "+label, "--body", prBody(t))
	cmd.Dir = t.Cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create --head %s: %w: %s", t.Branch, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// firstNeverCommit returns the first staged path docs/git-conventions.md §
// "What Never to Commit" forbids, or "". `git add -A` sweeps in every unignored
// file the spawn left behind and .gitignore does not cover all of them, so the
// landing refuses rather than committing a credential or a build artifact.
func firstNeverCommit(staged string) string {
	for _, path := range strings.Split(staged, "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		base := filepath.Base(path)
		switch {
		case base == ".env" || (strings.HasPrefix(base, ".env.") && base != ".env.example"),
			base == "auth.json", base == "coverage.out", strings.HasSuffix(base, ".test"):
			return path
		}
		for _, seg := range strings.Split(path, "/") {
			if seg == "bin" || seg == "dist" {
				return path
			}
		}
	}
	return ""
}

// prBody fills .github/PULL_REQUEST_TEMPLATE.md rather than emitting two bare
// lines: the template's Test Plan is marked REQUIRED, so the harness states what
// it actually knows — the gates that ran — instead of dropping the section.
func prBody(t core.Task) string {
	why := t.Title
	if why == "" {
		why = "See the issue's goal: " + t.ID + "."
	}
	return fmt.Sprintf(`Resolves %s

## What changed
Landed by `+"`anvil build`"+` from the verified worktree for %s — whatever the complete spawn left on `+"`%s`"+`.

## Why
%s

## How
Harness-landed (anvil.0162): the driver's advance-gate stages, commits and pushes the spawn's tree, then opens this PR. No author narrative was captured.

## Test Plan
The worker ran the issue's `+"`## Verification`"+` and the build gate before this landing; no author narrative was captured (harness-landed).

## Risk
None beyond the diff — the human owns the merge.
`, t.ID, t.ID, t.Branch, why)
}

// commitMessage labels the driver-landed commit. The PR squash-merges, so this
// is an audit line — it names the task whose verified tree it carries.
func commitMessage(t core.Task) string {
	return fmt.Sprintf("chore: land verified diff for %s", t.ID)
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
