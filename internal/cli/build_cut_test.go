package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// TestEngineCutWorktree_ClaimsFrontierOnDeterministicBranch verifies the build
// driver's claim+cut: each ready issue is transitioned in-progress under the
// anvil-build owner and its canonical worktree (<project>/<slug>) is cut from
// origin/HEAD and pinned as the matching task's Cwd — so the spawned worker
// lands its PR on the deterministic branch the driver already holds.
func TestEngineCutWorktree_ClaimsFrontierOnDeterministicBranch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t) // demo.foo: open, ready, goal set

	s := stubSideFX(t) // fakes git/gh; homeDir=/home/anvil, originHEAD=origin/master

	v, err := core.ResolveVault()
	if err != nil {
		t.Fatalf("resolve vault: %v", err)
	}
	units := []readyUnit{{ID: "demo.foo", Path: filepath.Join(vault, "70-issues", "demo.foo.md")}}
	tasks := readyUnitsToTasks(units)

	if err := claimAndCutForBuild(v, &bytes.Buffer{}, units, tasks); err != nil {
		t.Fatalf("claimAndCutForBuild: %v", err)
	}

	wantPath := filepath.Join("/home/anvil", "Development", "demo-worktrees", "foo")
	if len(s.addCalls) != 1 {
		t.Fatalf("git worktree add calls = %d, want 1: %+v", len(s.addCalls), s.addCalls)
	}
	if got := s.addCalls[0]; got.Branch != "demo/foo" || got.Path != wantPath || got.StartPoint != "origin/master" {
		t.Errorf("cut = %+v; want branch demo/foo, path %s, startpoint origin/master", got, wantPath)
	}
	if tasks[0].Cwd != wantPath {
		t.Errorf("task Cwd = %q, want pinned worktree %q", tasks[0].Cwd, wantPath)
	}

	a := loadIssueDoc(t, vault, "demo.foo")
	if got, _ := a.FrontMatter["status"].(string); got != "in-progress" {
		t.Errorf("issue status = %q, want in-progress", got)
	}
	if got, _ := a.FrontMatter["owner"].(string); got != buildClaimOwner {
		t.Errorf("issue owner = %q, want %s", got, buildClaimOwner)
	}
}
