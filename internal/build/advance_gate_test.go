package build

import (
	"context"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// TestEnsurePRForTask_RequiresWorktree: the engine treats an empty Task.Cwd as
// legal (it falls back to Options.Cwd), but the gate commits and pushes — with
// no Cwd every git command would run in the driver's own checkout, on master.
// CLAUDE.md forbids that, so the gate refuses before touching git.
func TestEnsurePRForTask_RequiresWorktree(t *testing.T) {
	task := core.Task{ID: "anvil.0162.nocwd", Branch: "anvil/0162-nocwd"}

	ok, err := EnsurePRForTask(context.Background(), task)
	if ok {
		t.Error("gate passed a task with no worktree")
	}
	if err == nil {
		t.Fatal("gate accepted a task with no worktree to land from")
	}
	if !strings.Contains(err.Error(), "no worktree") {
		t.Errorf("error does not name the missing worktree: %v", err)
	}
}
