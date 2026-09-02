package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

// worktreeHookWaitDelay bounds Wait when a killed hook's orphaned child
// still holds the stderr pipe — otherwise the refusal is correct but
// unbounded in wall-clock time.
const worktreeHookWaitDelay = 5 * time.Second

// worktreeHookWaitDelayFn: worktreeHookTimeoutFn's counterpart, also test-shrinkable.
var worktreeHookWaitDelayFn = func() time.Duration { return worktreeHookWaitDelay }

var runWorktreeHookFn = runWorktreeHookReal

// runWorktreeHookReal runs repoDir's worktreeHookName if present, synchronously.
// Missing hook is a no-op; a present-but-failing one (incl. timeout) returns
// Structured — the caller cleans up the worktree and branch.
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
	// Setpgid below detaches the hook into its own group, so forward
	// interrupts explicitly or Ctrl-C on anvil leaves it running unsupervised.
	sigCtx, stopNotify := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopNotify()
	ctx, cancel := context.WithTimeout(sigCtx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, hookPath, worktreePath) //nolint:gosec // G204: fixed repo-relative name, executable bit checked above, bounded by worktreeHookTimeout
	cmd.Dir = repoDir
	// Setpgid + group-kill Cancel make the process group the unit: a timeout
	// or interrupt must kill every descendant, not just the direct child, or
	// an orphan keeps the stderr pipe open and wedges Wait for the full
	// WaitDelay.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = worktreeHookWaitDelayFn()
	stderr := &boundedBuffer{capBytes: tailByteCap}
	cmd.Stderr = stderr
	runErr := cmd.Run()
	// Any non-nil error may leave an orphan holding stderr — kill the group
	// unconditionally, not just on the timeout/ErrWaitDelay branches below.
	if runErr != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if runErr != nil && errors.Is(runErr, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		// Exited 0 but an orphaned descendant held stderr past WaitDelay —
		// refuse with a named code, not Go's internal WaitDelay string.
		return errfmt.NewStructured("cut_worktree_hook_orphan_process").
			Set("hook", hookPath).
			Set("fix_hint", "the hook must spawn nothing that outlives it")
	}
	if runErr != nil {
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

// tailByteCap bounds both boundedBuffer's stored bytes and tailLines' line-
// splitting input — a hook that floods stderr must not buffer unbounded
// output into memory or the refusal.
const tailByteCap = 4 * 1024

// boundedBuffer is an io.Writer that keeps only the last capBytes written —
// cmd.Stderr must not accumulate a flooding hook's output unbounded just
// because the refusal only ever surfaces a tail of it.
type boundedBuffer struct {
	buf      []byte
	capBytes int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.capBytes {
		b.buf = b.buf[len(b.buf)-b.capBytes:]
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string { return string(b.buf) }

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
