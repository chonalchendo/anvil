package index

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertArtifactInsertsThenUpdates(t *testing.T) {
	db := openTestDB(t)
	row := ArtifactRow{ID: "demo.foo", Type: "issue", Status: "open", Project: "demo", Path: "/p/foo.md"}
	if err := db.UpsertArtifact(row); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	row.Status = "in-progress"
	if err := db.UpsertArtifact(row); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := db.GetArtifact("demo.foo")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if got.Status != "in-progress" {
		t.Fatalf("status: got %q want %q", got.Status, "in-progress")
	}
}

func TestReplaceLinksRemovesPriorEdges(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertArtifact(ArtifactRow{ID: "a", Type: "issue", Path: "/p/a.md"}); err != nil {
		t.Fatal(err)
	}
	first := []LinkRow{
		{Source: "a", Target: "b", Relation: "milestone"},
		{Source: "a", Target: "c", Relation: "related"},
	}
	if err := db.ReplaceLinks("a", first); err != nil {
		t.Fatalf("replace 1: %v", err)
	}
	second := []LinkRow{{Source: "a", Target: "d", Relation: "milestone"}}
	if err := db.ReplaceLinks("a", second); err != nil {
		t.Fatalf("replace 2: %v", err)
	}

	got, err := db.LinksFrom("a")
	if err != nil {
		t.Fatalf("LinksFrom: %v", err)
	}
	if diff := cmp.Diff(second, got, cmpopts.SortSlices(func(x, y LinkRow) bool { return x.Target < y.Target })); diff != "" {
		t.Fatalf("links mismatch (-want +got):\n%s", diff)
	}
}

func TestDeleteArtifactRemovesLinks(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertArtifact(ArtifactRow{ID: "a", Type: "issue", Path: "/p/a.md"}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceLinks("a", []LinkRow{{Source: "a", Target: "b", Relation: "related"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteArtifact("a"); err != nil {
		t.Fatalf("DeleteArtifact: %v", err)
	}
	got, err := db.LinksFrom("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("links not removed: %v", got)
	}
	if _, err := db.GetArtifact("a"); err == nil {
		t.Fatalf("expected GetArtifact to fail after delete")
	}
}
