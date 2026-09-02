package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

// worktreeHookName: repo-root escape hatch for state carry can't copy
// (anvil.0270). cwd=repo root, $1=absolute worktree path.
const worktreeHookName = ".anvil-worktree-hook"

// worktreeHookTimeout caps a single hook run, alongside anchorTimeout and
// feasibilityTimeout. A hung hook must not wedge the transition indefinitely.
const worktreeHookTimeout = 30 * time.Second

// worktreeHookTimeoutFn resolves the timeout to apply — a var seam (mirrors
// gitWorktreeAddFn et al.) so a test can shrink it instead of sleeping out
// the full 30s to exercise the timeout path for real.
var worktreeHookTimeoutFn = func() time.Duration { return worktreeHookTimeout }

var runWorktreeHookFn = runWorktreeHookReal

// runWorktreeHookReal runs repoDir's worktreeHookName if present. Synchronous:
// a process outliving the call wedges a later worktree remove. Missing hook is
// a no-op; a present-but-failing one (incl. timeout) returns Structured — the
// caller cleans up the worktree and branch.
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
	timeout := worktreeHookTimeoutFn()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, hookPath, worktreePath) //nolint:gosec // G204: fixed repo-relative name, executable bit checked above, bounded by worktreeHookTimeout
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errfmt.NewStructured("cut_worktree_hook_timeout").
				Set("hook", hookPath).
				Set("timeout", timeout.String()).
				Set("stderr_tail", tailLines(stderr.String(), 20))
		}
		return errfmt.NewStructured("cut_worktree_hook_failed").
			Set("hook", hookPath).
			Set("error", runErr.Error()).
			Set("stderr_tail", tailLines(stderr.String(), 20))
	}
	return nil
}

// tailByteCap bounds tailLines' input before line-splitting — a hook that
// floods stderr must not buffer unbounded output into the refusal.
const tailByteCap = 4 * 1024

// tailLines returns the last n lines of the last tailByteCap bytes of s,
// trimmed — enough to diagnose a failing hook without dumping unbounded
// output into the refusal.
func tailLines(s string, n int) string {
	truncated := false
	if len(s) > tailByteCap {
		s = s[len(s)-tailByteCap:]
		truncated = true
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
		truncated = true
	}
	out := strings.Join(lines, "\n")
	if truncated {
		out = "...(truncated)\n" + out
	}
	return out
}
