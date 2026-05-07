package index

import (
	"os"
	"path/filepath"
	"testing"
)

func writeArtifact(t *testing.T, path, fm string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" + fm + "---\n\nbody\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReindexPopulatesArtifactsAndLinks(t *testing.T) {
	vault := t.TempDir()
	writeArtifact(t, filepath.Join(vault, "70-issues", "demo.foo.md"),
		"type: issue\nid: demo.foo\nproject: demo\nstatus: open\nmilestone: \"[[milestone.demo.m1]]\"\n")
	writeArtifact(t, filepath.Join(vault, "85-milestones", "demo.m1.md"),
		"type: milestone\nid: demo.m1\nproject: demo\nstatus: planned\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stats, err := db.Reindex(vault)
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if stats.Artifacts != 2 {
		t.Fatalf("artifacts: got %d want 2", stats.Artifacts)
	}
	if stats.Links != 1 {
		t.Fatalf("links: got %d want 1", stats.Links)
	}

	if _, err := db.GetArtifact("demo.foo"); err != nil {
		t.Fatalf("expected demo.foo present: %v", err)
	}
	if _, err := db.GetLastReindex(); err != nil {
		t.Fatalf("last reindex stamp not set: %v", err)
	}
}

func TestReindexIsIdempotent(t *testing.T) {
	vault := t.TempDir()
	writeArtifact(t, filepath.Join(vault, "70-issues", "a.md"),
		"type: issue\nid: a\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Reindex(vault); err != nil {
		t.Fatal(err)
	}
	stats, err := db.Reindex(vault)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Artifacts != 1 {
		t.Fatalf("artifacts: got %d want 1", stats.Artifacts)
	}
}
