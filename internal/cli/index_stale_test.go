package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// markVaultExternallyStale plants a non-anvil .md file and bumps the vault
// dir mtime so CheckFreshness reports index_stale on the next read.
func markVaultExternallyStale(t *testing.T, vault, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(vault, "70-issues", name),
		[]byte("---\ntype: issue\nid: demo.external\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(vault, now, now); err != nil {
		t.Fatal(err)
	}
}

func TestCreateRollsBackOnIndexStale(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	markVaultExternallyStale(t, vault, "demo.external.md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--project", "demo", "--title", "fresh",
		"--description", "fresh desc",
		"--tags", "domain/dev-tools"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected index_stale; output: %s", out.String())
	}

	if _, err := os.Stat(filepath.Join(vault, "70-issues", "demo.fresh.md")); !os.IsNotExist(err) {
		t.Fatalf("expected demo.fresh.md to be rolled back; stat err = %v", err)
	}
}

func TestCreateUpdateRollsBackOnIndexStale(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	planPath := filepath.Join(vault, "70-issues", "demo.foo.md")
	originalBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}

	markVaultExternallyStale(t, vault, "demo.external.md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--project", "demo", "--title", "foo",
		"--description", "rewritten desc",
		"--tags", "domain/dev-tools", "--update"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected index_stale on --update; output: %s", out.String())
	}

	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, originalBytes) {
		t.Errorf("expected original bytes preserved on rollback;\nwant:\n%s\ngot:\n%s", originalBytes, got)
	}
}

func TestCreateRetryAfterIndexStaleSucceedsIdempotently(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	// Stale-trigger: write external file + advance vault dir mtime just past
	// the current stamp. Keep the offset tiny so a follow-up reindex's stamp
	// naturally eclipses it without sleeping.
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.external.md"),
		[]byte("---\ntype: issue\nid: demo.external\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bumped := time.Now().Add(10 * time.Millisecond)
	if err := os.Chtimes(vault, bumped, bumped); err != nil {
		t.Fatal(err)
	}

	// First attempt fails with index_stale.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--project", "demo", "--title", "fresh",
		"--description", "fresh desc",
		"--tags", "domain/dev-tools"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected first attempt to fail; output: %s", out.String())
	}

	// User runs reindex to absorb the external edit.
	time.Sleep(20 * time.Millisecond)
	execCmd(t, "reindex")

	// Retry with the same args — must succeed (not "drift" or "already_exists").
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "fresh",
		"--description", "fresh desc",
		"--tags", "domain/dev-tools")

	if _, err := os.Stat(filepath.Join(vault, "70-issues", "demo.fresh.md")); err != nil {
		t.Fatalf("expected demo.fresh.md after retry: %v", err)
	}
}

func TestListReadyReturnsIndexStaleWhenVaultEditedExternally(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	// External edit + bump dir mtime so CheckFreshness sees drift.
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.bar.md"),
		[]byte("---\ntype: issue\nid: demo.bar\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(vault, now, now); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue", "--ready"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected ErrIndexStale; output: %s", out.String())
	}
	if !strings.Contains(err.Error(), "index_stale") {
		t.Fatalf("expected index_stale in error message: %v", err)
	}
}
