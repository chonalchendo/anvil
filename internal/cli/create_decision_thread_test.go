package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestCreateDecisionThread(t *testing.T) {
	vault := setupVault(t)
	t.Chdir(t.TempDir())

	tags := []string{"--description", "fixture", "--tags", "domain/cli,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"}
	create := func(args ...string) error {
		_, _, err := runCmd(t, newRootCmd(), append(args, tags...)...)
		return err
	}
	if err := create("create", "thread", "--title", "Which store?", "--topic", "demo"); err != nil {
		t.Fatal(err)
	}

	if err := create("create", "decision", "--title", "Use sqlite", "--thread", "demo.0001-which-store"); err != nil {
		t.Fatal(err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "30-decisions", "demo.0001-use-sqlite.md"))
	if err != nil {
		t.Fatalf("topic not inferred from thread: %v", err)
	}
	if got := a.FrontMatter["thread"]; got != "[[thread.demo.0001-which-store]]" {
		t.Errorf("thread slot = %v", got)
	}

	if err := create("create", "decision", "--title", "Use duckdb", "--thread", "demo.0009-nope"); err == nil {
		t.Error("dangling thread: want refusal")
	}
	files, _ := filepath.Glob(filepath.Join(vault, "30-decisions", "*duckdb*"))
	if len(files) != 0 {
		t.Errorf("dangling thread wrote %v", files)
	}

	if err := create("create", "thread", "--title", "Other", "--topic", "demo", "--thread", "demo.0001-which-store"); err == nil {
		t.Error("--thread on thread: want refusal")
	}
	if _, err := os.Stat(filepath.Join(vault, "60-threads", "demo.0002-other.md")); err == nil {
		t.Error("--thread on thread wrote a file")
	}
}
