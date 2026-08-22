package index

import (
	"os"
	"path/filepath"
	"testing"
)

// A vault addressed through a symlinked root must index identically to the direct
// path. filepath.WalkDir refuses to descend a symlinked root, so before the walk
// resolved it this returned zero artifacts with exit 0 — the whole vault
// silently invisible while reads kept serving a stale index.
func TestReindexFullThroughSymlinkedRootMatchesRealPath(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	writeArtifact(t, filepath.Join(store, "70-issues", "demo.foo.md"),
		"type: issue\nid: demo.foo\nproject: demo\nstatus: open\nmilestone: \"[[milestone.demo.m1]]\"\n")
	writeArtifact(t, filepath.Join(store, "85-milestones", "demo.m1.md"),
		"type: milestone\nid: demo.m1\nproject: demo\nstatus: planned\n")

	link := filepath.Join(dir, "link")
	if err := os.Symlink(store, link); err != nil {
		t.Fatal(err)
	}

	direct, err := reindexInto(t, store)
	if err != nil {
		t.Fatalf("reindex via direct path: %v", err)
	}
	if direct.Artifacts == 0 {
		t.Fatal("control indexed nothing; the fixture is wrong, not the walk")
	}

	linked, err := reindexInto(t, link)
	if err != nil {
		t.Fatalf("reindex via symlinked root: %v", err)
	}
	if linked.Artifacts != direct.Artifacts || linked.Links != direct.Links {
		t.Errorf("symlinked root indexed %d artifacts / %d links, want %d / %d as via the direct path",
			linked.Artifacts, linked.Links, direct.Artifacts, direct.Links)
	}
}

// reindexInto runs a full reindex of vaultRoot against its own database.
func reindexInto(t *testing.T, vaultRoot string) (ReindexStats, error) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	return db.ReindexFull(vaultRoot)
}
