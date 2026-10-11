package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestCreateDecisionThread(t *testing.T) {
	vault := setupVault(t)
	t.Chdir(t.TempDir())

	tags := []string{"--description", "fixture", "--tags", "domain/cli,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"}
	create := func(args ...string) (string, error) {
		out, _, err := runCmd(t, newRootCmd(), append(args, tags...)...)
		return out, err
	}
	if _, err := create("create", "thread", "--title", "Which store?", "--topic", "demo"); err != nil {
		t.Fatal(err)
	}

	forms := []string{"demo.0001-which-store", "thread.demo.0001-which-store", "[[thread.demo.0001-which-store]]"}
	for i, form := range forms {
		title := "Use sqlite " + string(rune('a'+i))
		if _, err := create("create", "decision", "--title", title, "--thread", form); err != nil {
			t.Fatalf("form %q: %v", form, err)
		}
		files, _ := filepath.Glob(filepath.Join(vault, "30-decisions", "demo.*-use-sqlite-"+string(rune('a'+i))+".md"))
		if len(files) != 1 {
			t.Fatalf("form %q: topic not inferred from thread, files = %v", form, files)
		}
		a, err := core.LoadArtifact(files[0])
		if err != nil {
			t.Fatal(err)
		}
		if got := a.FrontMatter["thread"]; got != "[[thread.demo.0001-which-store]]" {
			t.Errorf("form %q: thread slot = %v", form, got)
		}
	}

	if _, err := create("create", "decision", "--title", "Explicit topic", "--topic", "other", "--thread", "demo.0001-which-store"); err != nil {
		t.Fatal(err)
	}
	if files, _ := filepath.Glob(filepath.Join(vault, "30-decisions", "other.*-explicit-topic.md")); len(files) != 1 {
		t.Errorf("explicit --topic must win, files = %v", files)
	}

	out, err := create("create", "decision", "--title", "Use duckdb", "--thread", "demo.0009-nope", "--json")
	if err == nil {
		t.Error("dangling thread: want refusal")
	}
	for _, want := range []string{"unresolved_link", `"thread"`} {
		if !strings.Contains(out, want) {
			t.Errorf("dangling thread output missing %q: %s", want, out)
		}
	}
	if files, _ := filepath.Glob(filepath.Join(vault, "30-decisions", "*duckdb*")); len(files) != 0 {
		t.Errorf("dangling thread wrote %v", files)
	}

	out, err = create("create", "thread", "--title", "Other", "--topic", "demo", "--thread", "demo.0001-which-store", "--json")
	if err == nil {
		t.Error("--thread on thread: want refusal")
	}
	if !strings.Contains(out, "unsupported_flag_for_type") {
		t.Errorf("--thread on thread output missing unsupported_flag_for_type: %s", out)
	}
	if _, err := os.Stat(filepath.Join(vault, "60-threads", "demo.0002-other.md")); err == nil {
		t.Error("--thread on thread wrote a file")
	}
}
