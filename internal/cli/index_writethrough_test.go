package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/index"
)

// execCmd creates a fresh root command, runs it with args, and returns stdout+stderr.
// Fails the test if the command returns an error.
func execCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("anvil %v: %v\noutput: %s", args, err, out.String())
	}
	return out.String()
}

func openIndex(t *testing.T, vault string) *index.DB {
	t.Helper()
	db, err := index.Open(index.DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateWritesThroughToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)

	row, err := openIndex(t, vault).GetArtifact("demo.foo")
	if err != nil {
		t.Fatalf("expected demo.foo in index: %v", err)
	}
	if row.Type != "issue" {
		t.Fatalf("type: %q", row.Type)
	}
}

func TestSetStatusWritesThroughToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	execCmd(t, "set", "issue", "demo.foo", "status", "in-progress")

	row, err := openIndex(t, vault).GetArtifact("demo.foo")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "in-progress" {
		t.Fatalf("status: %q", row.Status)
	}
}

func TestLinkWritesThroughToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	execCmd(t, "create", "milestone",
		"--project", "demo",
		"--title", "m1",
		"--description", "m1 desc",
	)
	execCmd(t, "link", "issue", "demo.foo", "milestone", "demo.m1")

	rows, err := openIndex(t, vault).LinksFrom("demo.foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Target != "demo.m1" {
		t.Fatalf("links: %v", rows)
	}
}

func TestExternalEditMarksIndexStale(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)

	// External edit: write a new file directly to vault, bypassing the index.
	// Then bump the vault root's mtime so CheckFreshness sees the change.
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.bar.md"),
		[]byte("---\ntype: issue\nid: demo.bar\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(vault, now, now); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "demo.foo", "status", "in-progress"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected ErrIndexStale, got nil; output: %s", out.String())
	}
}
