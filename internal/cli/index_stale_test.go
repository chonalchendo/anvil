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

func TestCreateAbsorbsExternalDriftWithoutManualReindex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	markVaultExternallyStale(t, vault, "demo.external.md")

	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "fresh",
		"--description", "fresh desc",
		"--tags", "domain/dev-tools")

	if _, err := os.Stat(filepath.Join(vault, "70-issues", "demo.fresh.md")); err != nil {
		t.Fatalf("expected demo.fresh.md after auto-reindex: %v", err)
	}
	db := openIndex(t, vault)
	if _, err := db.GetArtifact("demo.fresh"); err != nil {
		t.Fatalf("demo.fresh missing from index: %v", err)
	}
	if _, err := db.GetArtifact("demo.external"); err != nil {
		t.Fatalf("external drift artifact not absorbed: %v", err)
	}
}

func TestCreateUpdateAbsorbsExternalDriftWithoutManualReindex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	markVaultExternallyStale(t, vault, "demo.external.md")

	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "foo",
		"--description", "rewritten desc",
		"--tags", "domain/dev-tools", "--update")

	got, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "rewritten desc") {
		t.Fatalf("expected rewritten desc; got:\n%s", got)
	}
	if _, err := openIndex(t, vault).GetArtifact("demo.external"); err != nil {
		t.Fatalf("external drift artifact not absorbed: %v", err)
	}
}

// Acceptance: two creates in one process, no intervening `anvil reindex`,
// both succeed. Mirrors the dogfood loop where each `anvil create` advances
// the vault dir mtime and previously tripped index_stale on the next write.
func TestTwoCreatesInOneProcessSucceedWithoutManualReindex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--project", "demo", "--title", "one",
		"--description", "first issue",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain"})
	var out1 bytes.Buffer
	cmd.SetOut(&out1)
	cmd.SetErr(&out1)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first create: %v\noutput: %s", err, out1.String())
	}

	// Plant an external edit between the two creates to force the stale
	// path; without auto-reindex this would error with [index_stale].
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.external.md"),
		[]byte("---\ntype: issue\nid: demo.external\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bumped := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(vault, bumped, bumped); err != nil {
		t.Fatal(err)
	}

	cmd = newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--project", "demo", "--title", "two",
		"--description", "second issue",
		"--tags", "domain/dev-tools"})
	var out2 bytes.Buffer
	cmd.SetOut(&out2)
	cmd.SetErr(&out2)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second create unexpectedly failed: %v\noutput: %s", err, out2.String())
	}

	db := openIndex(t, vault)
	for _, id := range []string{"demo.one", "demo.two", "demo.external"} {
		if _, err := db.GetArtifact(id); err != nil {
			t.Errorf("missing %s in index: %v", id, err)
		}
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
