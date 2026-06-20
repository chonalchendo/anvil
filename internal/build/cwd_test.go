package build

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// cwdRecorder is a minimal adapter that records the Cwd each task is dispatched
// into, so the per-task vs global Cwd routing can be asserted.
type cwdRecorder struct {
	mu   sync.Mutex
	cwds []string
}

func (c *cwdRecorder) Name() string { return "cwd-recorder" }

func (c *cwdRecorder) Run(_ context.Context, req RunRequest) (RunResult, error) {
	c.mu.Lock()
	c.cwds = append(c.cwds, req.Cwd)
	c.mu.Unlock()
	return RunResult{ExitCode: 0}, nil
}

// TestBuild_PerTaskCwd_PinsWorktreeOverGlobalCwd asserts the engine routes a
// task's own Cwd (the driver's pinned worktree) to the adapter, falling back to
// the global Options.Cwd only when the task carries none. This is the passthrough
// that lets a build worker land on the deterministic branch the driver cut.
func TestBuild_PerTaskCwd_PinsWorktreeOverGlobalCwd(t *testing.T) {
	rec := &cwdRecorder{}
	opts := Options{
		Cwd:    "/global/cwd",
		Stdout: io.Discard,
		Stderr: io.Discard,
		Router: Router{"claude-": rec},
	}
	waves := [][]core.Task{{
		{ID: "pinned", Model: "claude-sonnet-4-6", Body: "x", Cwd: "/worktrees/demo/pinned"},
		{ID: "fallback", Model: "claude-sonnet-4-6", Body: "y"},
	}}

	if _, err := Build(context.Background(), waves, opts); err != nil {
		t.Fatalf("Build: %v", err)
	}

	got := map[string]bool{}
	for _, c := range rec.cwds {
		got[c] = true
	}
	if !got["/worktrees/demo/pinned"] {
		t.Errorf("pinned task was not dispatched into its per-task Cwd; saw %v", rec.cwds)
	}
	if !got["/global/cwd"] {
		t.Errorf("Cwd-less task did not fall back to global Options.Cwd; saw %v", rec.cwds)
	}
}
