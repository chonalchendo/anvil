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
	if err := os.WriteFile(filepath.Join(vault, "70-issues", name), //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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

	path := createIssueGetPath(t,
		"create", "issue",
		"--project", "demo", "--title", "fresh",
		"--description", "fresh desc",
		"--goal", "fresh is done",
		"--tags", "domain/dev-tools",
	)
	freshID := strings.TrimSuffix(filepath.Base(path), ".md")

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected issue file after auto-reindex: %v", err)
	}
	db := openIndex(t, vault)
	if _, err := db.GetArtifact(freshID); err != nil {
		t.Fatalf("fresh issue missing from index: %v", err)
	}
	if _, err := db.GetArtifact("demo.external"); err != nil {
		t.Fatalf("external drift artifact not absorbed: %v", err)
	}
}

// TestCreateUpdateAbsorbsExternalDriftWithoutManualReindex: plans use
// deterministic slugs and support --update; verify the auto-reindex path
// when the vault has external drift.
func TestCreateUpdateAbsorbsExternalDriftWithoutManualReindex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// Seed a plan to rewrite via --update.
	issueFixturePath := writeFixtureIssueDated(t, vault, "demo", "foo", "foo", "2026-01-01")
	issueID := strings.TrimSuffix(filepath.Base(issueFixturePath), ".md")
	execCmd(t, "create", "plan",
		"--issue", "[[issue."+issueID+"]]",
		"--project", "demo", "--title", "foo",
		"--description", "original desc",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	markVaultExternallyStale(t, vault, "demo.external.md")

	execCmd(t, "create", "plan",
		"--issue", "[[issue."+issueID+"]]",
		"--project", "demo", "--title", "foo",
		"--description", "rewritten desc",
		"--tags", "domain/dev-tools", "--update")

	got, err := os.ReadFile(filepath.Join(vault, "80-plans", "demo.foo.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
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
	cmd.SetArgs([]string{
		"create", "issue",
		"--project", "demo", "--title", "one",
		"--description", "first issue",
		"--goal", "one is done",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var out1 bytes.Buffer
	cmd.SetOut(&out1)
	cmd.SetErr(&out1)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first create: %v\noutput: %s", err, out1.String())
	}

	// Plant an external edit between the two creates to force the stale
	// path; without auto-reindex this would error with [index_stale].
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.external.md"), //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		[]byte("---\ntype: issue\nid: demo.external\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bumped := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(vault, bumped, bumped); err != nil {
		t.Fatal(err)
	}

	cmd = newRootCmd()
	cmd.SetArgs([]string{
		"create", "issue",
		"--project", "demo", "--title", "two",
		"--description", "second issue",
		"--goal", "two is done",
		"--tags", "domain/dev-tools",
	})
	var out2 bytes.Buffer
	cmd.SetOut(&out2)
	cmd.SetErr(&out2)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second create unexpectedly failed: %v\noutput: %s", err, out2.String())
	}

	db := openIndex(t, vault)
	// Exact IDs come from the numbered format; just verify both issues are indexed.
	entries, _ := os.ReadDir(filepath.Join(vault, "70-issues"))
	var createdIDs []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "demo.") && strings.HasSuffix(e.Name(), ".md") {
			createdIDs = append(createdIDs, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	// Expect two created issues + the external one.
	if len(createdIDs) < 2 {
		t.Fatalf("expected at least 2 demo issue files; got %v", createdIDs)
	}
	for _, id := range createdIDs {
		if _, err := db.GetArtifact(id); err != nil {
			t.Errorf("missing %s in index: %v", id, err)
		}
	}
	if _, err := db.GetArtifact("demo.external"); err != nil {
		t.Fatalf("external drift not absorbed: %v", err)
	}
}

func TestListReadyReturnsIndexStaleWhenVaultEditedExternally(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	// External edit + bump dir mtime so CheckFreshness sees drift.
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.bar.md"), //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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
