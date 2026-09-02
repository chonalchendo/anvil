package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

// worktreeHookName names a repo-root, tracked, executable script `--cut-worktree`
// runs after carry lands — a project's escape hatch to derive worktree-local
// state carry can't express (e.g. writing a read-only agent principal into the
// worktree's own .env; anvil.0270). Runs with cwd at the repo root and the
// absolute worktree path as $1.
const worktreeHookName = ".anvil-worktree-hook"

var runWorktreeHookFn = runWorktreeHookReal

// runWorktreeHookReal stats repoDir's worktreeHookName and, when present, runs
// it synchronously (so the cut cannot return before it finishes — a live
// process outliving the call is exactly what wedges a later worktree removal)
// with worktreePath as $1. A missing hook is a silent no-op. A present but
// non-executable hook and a non-zero exit both return a Structured refusal;
// the caller is responsible for removing the worktree and branch a failed
// hook leaves behind.
func runWorktreeHookReal(repoDir, worktreePath string) error {
	hookPath := filepath.Join(repoDir, worktreeHookName)
	info, err := os.Stat(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errfmt.NewStructured("cut_worktree_hook_failed").
			Set("hook", hookPath).
			Set("error", err.Error())
	}
	if info.Mode()&0o111 == 0 {
		return errfmt.NewStructured("cut_worktree_hook_not_executable").
			Set("hook", hookPath).
			Set("fix_hint", "chmod +x "+worktreeHookName+" or remove it")
	}
	cmd := exec.Command(hookPath, worktreePath) //nolint:gosec // G204: fixed repo-relative name, executable bit checked above
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return errfmt.NewStructured("cut_worktree_hook_failed").
			Set("hook", hookPath).
			Set("error", runErr.Error()).
			Set("stderr_tail", tailLines(stderr.String(), 20))
	}
	return nil
}

// tailLines returns the last n lines of s, trimmed — enough to diagnose a
// failing hook without dumping unbounded output into the refusal.
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
