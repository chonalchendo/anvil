package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/index"
)

func jsonUnmarshal(t *testing.T, s string, v any) error {
	t.Helper()
	return json.Unmarshal([]byte(s), v)
}

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
	t.Cleanup(func() { db.Close() }) //nolint:errcheck,gosec // close in defer; error not actionable
	return db
}

func TestCreateWritesThroughToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	// Use create issue with --json to capture the allocated numbered ID.
	out := execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--goal", "foo is done",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json",
	)
	var result map[string]any
	if err := jsonUnmarshal(t, out, &result); err != nil {
		t.Fatal(err)
	}
	id, _ := result["id"].(string)

	row, err := openIndex(t, vault).GetArtifact(id)
	if err != nil {
		t.Fatalf("expected %s in index: %v", id, err)
	}
	if row.Type != "issue" {
		t.Fatalf("type: %q", row.Type)
	}
}

func TestCreateWritesThroughTagsToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// Establish a last_reindex stamp first, so the create below takes the
	// write-through path (not the bootstrap full-reindex, which indexes tags
	// anyway and would hide the gap this test guards).
	execCmd(t, "reindex")

	out := execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "tagged",
		"--description", "tagged desc",
		"--goal", "tagged is done",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json",
	)
	var result map[string]any
	if err := jsonUnmarshal(t, out, &result); err != nil {
		t.Fatal(err)
	}
	id, _ := result["id"].(string)

	// No reindex: the create write-through must have populated the tags table,
	// so a tag query finds the new artifact immediately.
	rows, err := openIndex(t, vault).RelatedByTags([]string{"domain/dev-tools"}, index.QueryFilters{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("create did not write tags through to the index: %s absent from %v", id, rows)
	}
}

func TestCreateWritesThroughLearningFTSToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// Stamp last_reindex first, so the create takes the write-through path (not
	// the bootstrap full-reindex, which populates learning_fts anyway and would
	// mask the gap this test guards).
	execCmd(t, "reindex")

	out := execCmd(t, "create", "learning",
		"--title", "fts canary",
		"--tags", "domain/dev-tools,activity/governance",
		"--allow-new-facet", "domain,activity",
		"--body", "## TL;DR\nzqxwvcanary immediate searchability check.\n\n## Evidence\nn/a\n\n## Caveats\nn/a",
		"--json",
	)
	var result map[string]any
	if err := jsonUnmarshal(t, out, &result); err != nil {
		t.Fatal(err)
	}
	id, _ := result["id"].(string)

	// No reindex: the create write-through must have populated learning_fts, so
	// a content search over the TL;DR finds the new learning immediately.
	hits, err := openIndex(t, vault).SearchLearnings("zqxwvcanary", index.QueryFilters{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range hits {
		if r.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("create did not write learning TL;DR through to learning_fts: %s absent from %v", id, hits)
	}
}

func TestSetStatusWritesThroughToIndex(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureIssueDated(t, vault, "demo", "foo", "foo", "2026-01-01")
	execCmd(t, "reindex")
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
	writeFixtureIssueDated(t, vault, "demo", "foo", "foo", "2026-01-01")
	execCmd(t, "reindex")
	execCmd(t, "create", "milestone",
		"--project", "demo",
		"--title", "m1",
		"--description", "m1 desc",
		"--goal", "m1 ships and all attached issues are resolved",
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

func TestExternalEditAbsorbedOnNextWrite(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureIssueDated(t, vault, "demo", "foo", "foo", "2026-01-01")
	execCmd(t, "reindex")

	// External edit: write a new file directly to vault, bypassing the index.
	// Then bump the vault root's mtime so CheckFreshness sees the change.
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.bar.md"), //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		[]byte("---\ntype: issue\nid: demo.bar\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(vault, now, now); err != nil {
		t.Fatal(err)
	}

	// The next write through indexAfterSave auto-reindexes, absorbing the
	// external file so the user is not forced to run `anvil reindex` first.
	execCmd(t, "set", "issue", "demo.foo", "status", "in-progress")

	db := openIndex(t, vault)
	if row, err := db.GetArtifact("demo.foo"); err != nil || row.Status != "in-progress" {
		t.Fatalf("expected demo.foo in-progress; row=%+v err=%v", row, err)
	}
	if _, err := db.GetArtifact("demo.bar"); err != nil {
		t.Fatalf("external demo.bar not absorbed by auto-reindex: %v", err)
	}
}
