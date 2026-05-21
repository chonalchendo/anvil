package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// Function-typed package vars let tests swap external command execution
// without shimming binaries on PATH. Pattern mirrors ghPRListFn. The
// gitWorktreeListFn used here is declared once in fleet.go.
var (
	gitWorktreeAddFn    = gitWorktreeAddReal
	gitWorktreeRemoveFn = gitWorktreeRemoveReal
	gitMainRootFn       = gitMainRootReal
	ghPRViewJSONFn      = ghPRViewJSONReal
	ghPRChecksFn        = ghPRChecksReal
	ghPRMergeFn         = ghPRMergeReal
	userHomeFn          = os.UserHomeDir
)

func projectFromArtifact(a *core.Artifact, id string) string {
	if p, _ := a.FrontMatter["project"].(string); p != "" {
		return p
	}
	if dot := strings.IndexByte(id, '.'); dot > 0 {
		return id[:dot]
	}
	return ""
}

func slugFromIssueID(id string) string {
	dot := strings.IndexByte(id, '.')
	if dot < 0 || dot+1 >= len(id) {
		return id
	}
	return id[dot+1:]
}

func defaultWorktreePath(project, slug string) (string, error) {
	home, err := userHomeFn()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Development", project+"-worktrees", slug), nil
}

func gitWorktreeAddReal(repoDir, path, branch string) error {
	cmd := exec.Command("git", "worktree", "add", path, "-b", branch)
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitWorktreeRemoveReal(repoDir, path string) error {
	cmd := exec.Command("git", "worktree", "remove", path)
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// cutWorktreeIfNeeded creates `git worktree add path -b branch` unless an
// entry already matches (idempotent). Errors on path/branch mismatch with an
// existing worktree, or on git failure. Uses fleet.go's gitWorktreeListFn
// (branch-keyed map).
func cutWorktreeIfNeeded(path, branch string) error {
	worktrees, err := gitWorktreeListFn()
	if err != nil {
		return err
	}
	if info, ok := worktrees[branch]; ok {
		if info.path == path {
			return nil
		}
		return fmt.Errorf("branch %q already checked out at %s (expected %s)", branch, info.path, path)
	}
	for b, info := range worktrees {
		if info.path == path {
			return fmt.Errorf("worktree at %s already on branch %q (expected %q)", path, b, branch)
		}
	}
	return gitWorktreeAddFn("", path, branch)
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

func ghPRViewJSONReal(num int, fields string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errGhUnavailable
	}
	return exec.Command("gh", "pr", "view", strconv.Itoa(num), "--json", fields).Output()
}

// ghPRChecksReal runs `gh pr checks <num> --required` so optional pending
// checks (e.g. an unfinished CodeRabbit pass) don't gate the merge — only
// the required-checks suite enforces refusal.
func ghPRChecksReal(num int) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "checks", strconv.Itoa(num), "--required")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh pr checks: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ghPRMergeReal(num int) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "merge", strconv.Itoa(num), "--squash", "--delete-branch")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh pr merge: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// doCutWorktree resolves defaults from the issue, applies overrides, and
// cuts the worktree. Returns a Structured error on failure so callers can
// uniformly refuse the transition without writing to disk.
func doCutWorktree(a *core.Artifact, id, pathOverride, branchOverride string) error {
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	if project == "" || slug == "" {
		return errfmt.NewStructured("cut_worktree_path_failed").
			Set("error", "issue id lacks `<project>.<slug>` shape").
			Set("id", id)
	}
	wtPath := pathOverride
	if wtPath == "" {
		p, derr := defaultWorktreePath(project, slug)
		if derr != nil {
			return errfmt.NewStructured("cut_worktree_path_failed").Set("error", derr.Error())
		}
		wtPath = p
	}
	branch := branchOverride
	if branch == "" {
		branch = project + "/" + slug
	}
	if err := cutWorktreeIfNeeded(wtPath, branch); err != nil {
		return errfmt.NewStructured("cut_worktree_failed").
			Set("path", wtPath).
			Set("branch", branch).
			Set("error", err.Error())
	}
	return nil
}

// doLandPR derives the worktree path from the issue and runs landPR. Path
// derivation is a hard error: the audit line claims "worktree removed" and we
// refuse to lie if we can't compute the location.
func doLandPR(a *core.Artifact, id string, prNum int) error {
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	if project == "" || slug == "" {
		return errfmt.NewStructured("land_pr_path_failed").
			Set("error", "issue id lacks `<project>.<slug>` shape").
			Set("id", id)
	}
	wtPath, derr := defaultWorktreePath(project, slug)
	if derr != nil {
		return errfmt.NewStructured("land_pr_path_failed").Set("error", derr.Error())
	}
	return landPR(prNum, wtPath)
}

// landPR runs gate→remove→merge→verify. Returns nil on success or a Structured
// error keyed on the failing gate. Order matches docs/worktree-workflow.md:
// the worktree must be removed before `gh pr merge --delete-branch` will land
// the branch delete (gh refuses the delete while a worktree references it).
func landPR(num int, worktreePath string) error {
	type mergeState struct {
		Mergeable        string `json:"mergeable"`
		MergeStateStatus string `json:"mergeStateStatus"`
	}
	raw, err := ghPRViewJSONFn(num, "mergeable,mergeStateStatus")
	if err != nil {
		return errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	var st mergeState
	if err := json.Unmarshal(raw, &st); err != nil {
		return errfmt.NewStructured("land_pr_view_failed").Set("pr", num).Set("error", err.Error())
	}
	if st.Mergeable != "MERGEABLE" {
		return errfmt.NewStructured("land_pr_not_mergeable").
			Set("pr", num).
			Set("mergeable", st.Mergeable).
			Set("merge_state", st.MergeStateStatus)
	}
	if err := ghPRChecksFn(num); err != nil {
		return errfmt.NewStructured("land_pr_ci_not_green").Set("pr", num).Set("error", err.Error())
	}
	if worktreePath != "" {
		if _, ferr := os.Stat(worktreePath); ferr == nil {
			root, rerr := gitMainRootFn()
			if rerr != nil {
				return errfmt.NewStructured("land_pr_main_worktree_lookup_failed").Set("error", rerr.Error())
			}
			if err := gitWorktreeRemoveFn(root, worktreePath); err != nil {
				return errfmt.NewStructured("land_pr_worktree_remove_failed").
					Set("pr", num).
					Set("path", worktreePath).
					Set("error", err.Error())
			}
		}
	}
	if err := ghPRMergeFn(num); err != nil {
		return errfmt.NewStructured("land_pr_merge_failed").Set("pr", num).Set("error", err.Error())
	}
	type finalState struct {
		State string `json:"state"`
	}
	raw, err = ghPRViewJSONFn(num, "state")
	if err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	var fin finalState
	if err := json.Unmarshal(raw, &fin); err != nil {
		return errfmt.NewStructured("land_pr_state_verify_failed").Set("pr", num).Set("error", err.Error())
	}
	if fin.State != "MERGED" {
		return errfmt.NewStructured("land_pr_state_not_merged").Set("pr", num).Set("state", fin.State)
	}
	return nil
}
