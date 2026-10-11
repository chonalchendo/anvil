package cli

// Process-executing git shims behind the gitXxxFn seams declared in
// transition_side_effects.go. Split out by concern: everything here shells
// out to git; the side-effects file owns the orchestration around them.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitWorktreeAddReal creates a new worktree at path on branch, branching from
// startPoint when non-empty (e.g. "origin/HEAD"). An empty startPoint lets git
// branch from the current HEAD, which is the legacy behaviour.
func gitWorktreeAddReal(repoDir, path, branch, startPoint string) error {
	args := []string{"worktree", "add", path, "-b", branch}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	cmd := exec.Command("git", args...) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitFetchOriginReal runs `git fetch origin` from repoDir (the resolved
// project repo), not the caller's ambient cwd.
func gitFetchOriginReal(repoDir string) error {
	cmd := exec.Command("git", "fetch", "origin") //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitResolveOriginHEADReal resolves the symbolic ref origin/HEAD to a concrete
// remote-tracking ref (e.g. "origin/master"), run from repoDir. Returns an
// error when the remote does not exist or has no HEAD — the caller falls back
// to local HEAD.
func gitResolveOriginHEADReal(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "origin/HEAD") //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse origin/HEAD: %w", err)
	}
	ref := strings.TrimSpace(string(out))
	if ref == "" {
		return "", errors.New("git rev-parse origin/HEAD: empty output")
	}
	return ref, nil
}

// gitOriginBranchExistsReal reports whether the remote-tracking ref
// origin/<branch> exists in repoDir. Run after a fetch so it reflects origin.
func gitOriginBranchExistsReal(repoDir, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch) //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// gitLocalBranchExistsReal reports whether refs/heads/<branch> exists in repoDir.
func gitLocalBranchExistsReal(repoDir, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch) //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// resolveProjectRepoReal resolves the on-disk repo for a project via the
// `~/Development/<project>` convention (the sibling of
// `~/Development/<project>-worktrees` used by defaultWorktreePath). Refuses
// with an error rather than silently falling back to the caller's cwd — the
// bug this guards against is a worktree silently cut from whatever repo the
// invoking session happens to be standing in.
//
// The cwd is accepted only when it *is* the project's repo (toplevel basename
// == project), which covers a checkout living outside the convention path
// without reopening the wrong-repo cut.
func resolveProjectRepoReal(project string) (string, error) {
	home, err := userHomeFn()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Development", project)
	if info, serr := os.Stat(dir); serr != nil || !info.IsDir() {
		if top, terr := gitToplevelFn(); terr == nil && filepath.Base(top) == project {
			return top, nil
		}
		return "", fmt.Errorf("project repo not found at %s (expected `~/Development/%s`)", dir, project)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return "", fmt.Errorf("%s is not a git repo (no .git)", dir)
	}
	return dir, nil
}

// gitToplevelReal returns the repo root of the caller's cwd.
func gitToplevelReal() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output() //nolint:gosec // binary path resolved from trusted sources; not user input
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitWorktreeRemoveReal(repoDir, path string) error {
	cmd := exec.Command("git", "worktree", "remove", path) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitWorktreeRemoveForceReal is gitWorktreeRemoveReal's --force variant: a
// plain remove refuses once the hook wrote a non-gitignored file; land-pr
// stays non-force so uncommitted work isn't discarded.
func gitWorktreeRemoveForceReal(repoDir, path string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", path) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove --force: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitDeleteLocalBranchReal deletes the local branch via `git branch -D`, run
// from repoDir (the main worktree) so it does not depend on the caller's cwd —
// which may be the worktree just removed. Run after the worktree is removed so
// git does not refuse the delete on a branch still referenced by a worktree.
// Restores the local-branch cleanup that the old `gh pr merge --delete-branch`
// provided before this path split into a remote-only `gh api` delete.
func gitDeleteLocalBranchReal(repoDir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch) //nolint:gosec // binary path resolved from trusted sources; not user input
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -D %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitMainRootReal returns the main worktree's root directory by deriving it
// from `git rev-parse --git-common-dir`. The common dir is `<main>/.git` for
// every non-bare checkout, so its parent is the main worktree. Used as a
// stable cwd for `git worktree remove` when the caller's cwd is the worktree
// being removed.
func gitMainRootReal() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return "", errors.New("git rev-parse --git-common-dir: empty output")
	}
	return filepath.Dir(common), nil
}

// gitLayerPatchIDReal returns the stable patch-id of the layer diff: the
// merge-base of base and commit, to commit. With fork set, the merge-base is
// the fork point of base, so a rebased lower layer is excluded from an old
// upper commit; it falls back to the plain merge-base when no fork point exists.
func gitLayerPatchIDReal(dir, base, commit string, fork bool) (string, error) {
	mb, err := gitOutput(dir, "merge-base", base, commit)
	if err != nil {
		return "", err
	}
	mb = strings.TrimSpace(mb)
	if fork {
		// The fork point is trusted only when every commit it hides from the
		// plain merge-base has an equivalent in base. Otherwise a commit dropped
		// from the base would be excluded from the diff and mask a change.
		if out, ferr := gitOutput(dir, "merge-base", "--fork-point", base, commit); ferr == nil {
			if fp := strings.TrimSpace(out); fp != "" {
				if cherry, cerr := gitOutput(dir, "cherry", base, fp, mb); cerr == nil && !strings.Contains(cherry, "+") {
					mb = fp
				}
			}
		}
	}
	diff := exec.Command("git", "diff", "--no-ext-diff", "--no-color", "--no-textconv", mb, commit) //nolint:gosec // refs come from gh and the vault, passed as separate argv entries
	diff.Dir = dir
	raw, err := diff.Output()
	if err != nil {
		return "", err
	}
	pid := exec.Command("git", "patch-id", "--stable")
	pid.Dir = dir
	pid.Stdin = bytes.NewReader(raw)
	out, err := pid.Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("empty layer diff")
	}
	return fields[0], nil
}
