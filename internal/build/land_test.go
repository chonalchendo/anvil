package build

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// fakeGH answers the two gh calls the advance-gate makes: `pr list` reports no
// open PR until `pr create` runs, which records its argv. That makes "the driver
// opened the PR" an observable, without network or auth.
const fakeGH = `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "list" ]; then
  if [ -f "$GH_STATE/created" ]; then echo '[{"number":1}]'; else echo '[]'; fi
  exit 0
fi
if [ "$1" = "pr" ] && [ "$2" = "create" ]; then
  echo "$@" > "$GH_STATE/created"
  exit 0
fi
echo "unexpected gh invocation: $*" >&2
exit 1
`

// editorAdapter mirrors the 2026-07-05 worker that broke this: it edits a file
// in its worktree, exits 0, and never commits or opens a PR.
type editorAdapter struct{ file, content string }

func (e *editorAdapter) Name() string { return "editor" }

func (e *editorAdapter) Run(_ context.Context, req RunRequest) (RunResult, error) {
	if err := os.WriteFile(filepath.Join(req.Cwd, e.file), []byte(e.content), 0o600); err != nil {
		return RunResult{ExitCode: 1}, err
	}
	return RunResult{ExitCode: 0}, nil
}

func mustRun(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...) //nolint:gosec // fixture args are test literals
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// gitFixture builds a bare origin plus a clone checked out on branch, with a
// fake gh first on PATH. Returns the clone path and the fake gh's state dir,
// where a `pr create` invocation records its argv.
func gitFixture(t *testing.T, branch string) (clone, ghState string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	mustRun(t, root, "git", "init", "--bare", "--initial-branch=master", origin)

	clone = filepath.Join(root, "work")
	mustRun(t, root, "git", "clone", origin, clone)
	for _, kv := range [][2]string{{"user.email", "build@anvil.test"}, {"user.name", "anvil build"}, {"commit.gpgsign", "false"}} {
		mustRun(t, clone, "git", "config", kv[0], kv[1])
	}
	if err := os.WriteFile(filepath.Join(clone, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, clone, "git", "add", "-A")
	mustRun(t, clone, "git", "commit", "-m", "base")
	mustRun(t, clone, "git", "push", "-u", "origin", "master")
	mustRun(t, clone, "git", "remote", "set-head", "origin", "master")
	mustRun(t, clone, "git", "checkout", "-b", branch)

	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o700); err != nil { //nolint:gosec // test stub must be executable
		t.Fatal(err)
	}
	ghState = filepath.Join(root, "state")
	if err := os.MkdirAll(ghState, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_STATE", ghState)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return clone, ghState
}

// TestBuild_DriverLandsUncommittedVerifiedDiff is the anvil.0162 regression: a
// complete spawn that exits 0 leaving verified changes uncommitted still yields
// a committed branch and an opened PR, and the task classifies success.
func TestBuild_DriverLandsUncommittedVerifiedDiff(t *testing.T) {
	const branch = "anvil/0162-demo"
	clone, ghState := gitFixture(t, branch)

	opts := Options{
		Concurrency:    1,
		Stdout:         io.Discard,
		Stderr:         io.Discard,
		Router:         Router{"claude-": &editorAdapter{file: "landed.txt", content: "worker edit\n"}},
		VerifyArtifact: PRExistsForTask,
	}
	task := core.Task{ID: "anvil.0162.demo", Model: "claude-sonnet-4-6", Body: "edit", Cwd: clone, Branch: branch}

	sum, err := Build(context.Background(), [][]core.Task{{task}}, opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := sum.Outcomes[task.ID].Outcome; got != "success" {
		t.Fatalf("outcome = %q, want success (diagnostic: %s)", got, sum.Outcomes[task.ID].Result.Diagnostic)
	}
	if status := mustRun(t, clone, "git", "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Errorf("worktree still dirty after landing:\n%s", status)
	}
	if files := mustRun(t, clone, "git", "show", "--name-only", "--format=", "HEAD"); !strings.Contains(files, "landed.txt") {
		t.Errorf("worker edit not in the landed commit; HEAD touched:\n%s", files)
	}
	if remote := mustRun(t, clone, "git", "rev-parse", "origin/"+branch); strings.TrimSpace(remote) == "" {
		t.Error("branch was not pushed to origin")
	}
	created, err := os.ReadFile(filepath.Join(ghState, "created")) //nolint:gosec // ghState is the fixture's own t.TempDir
	if err != nil {
		t.Fatalf("gh pr create was never invoked: %v", err)
	}
	if !strings.Contains(string(created), branch) {
		t.Errorf("gh pr create did not target the driver's branch: %s", created)
	}
}

// TestBuild_NoDiffStaysFailed: landing must not manufacture an empty PR. A spawn
// that produced nothing keeps failing the advance-gate — the retry/nudge path
// owns it (anvil.0163).
func TestBuild_NoDiffStaysFailed(t *testing.T) {
	const branch = "anvil/0162-empty"
	clone, ghState := gitFixture(t, branch)

	opts := Options{
		Concurrency:    1,
		Stdout:         io.Discard,
		Stderr:         io.Discard,
		Router:         Router{"claude-": &fakeAdapter{}},
		VerifyArtifact: PRExistsForTask,
	}
	task := core.Task{ID: "anvil.0162.empty", Model: "claude-sonnet-4-6", Body: "nothing", Cwd: clone, Branch: branch}

	sum, _ := Build(context.Background(), [][]core.Task{{task}}, opts)
	if got := sum.Outcomes[task.ID].Outcome; got != "failed" {
		t.Fatalf("outcome = %q, want failed", got)
	}
	if _, err := os.Stat(filepath.Join(ghState, "created")); err == nil {
		t.Error("gh pr create ran for a spawn that produced no diff")
	}
}
