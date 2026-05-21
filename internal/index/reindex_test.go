package index

import (
	"os"
	"path/filepath"
	"testing"
)

func writeArtifact(t *testing.T, path, fm string) {
	t.Helper()
	writeArtifactBody(t, path, fm, "body")
}

func writeArtifactBody(t *testing.T, path, fm, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" + fm + "---\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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
	// One artifact with one outgoing wikilink so the second reindex has
	// something to (incorrectly, if buggy) duplicate.
	writeArtifact(t, filepath.Join(vault, "70-issues", "a.md"),
		"type: issue\nid: a\nstatus: open\nmilestone: \"[[milestone.m1]]\"\n")

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
	if stats.Links != 1 {
		t.Fatalf("links: got %d want 1 (second reindex should not duplicate)", stats.Links)
	}
	// Belt-and-braces: count the rows directly, not just the stats counter.
	var artifactRows, linkRows int
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM artifacts`).Scan(&artifactRows); err != nil {
		t.Fatal(err)
	}
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&linkRows); err != nil {
		t.Fatal(err)
	}
	if artifactRows != 1 || linkRows != 1 {
		t.Errorf("rows after second reindex: artifacts=%d links=%d, want 1/1", artifactRows, linkRows)
	}
}

func TestReindexBodyWikilinks(t *testing.T) {
	vault := t.TempDir()
	// Issue whose body references two other artifacts via wikilinks.
	writeArtifactBody(t, filepath.Join(vault, "70-issues", "anvil.src.md"),
		"type: issue\nid: anvil.src\nproject: anvil\nstatus: open\n",
		"See [[issue.anvil.foo]] and [[learning.anvil.bar]] for context.")
	writeArtifact(t, filepath.Join(vault, "70-issues", "anvil.foo.md"),
		"type: issue\nid: anvil.foo\nproject: anvil\nstatus: open\n")
	writeArtifact(t, filepath.Join(vault, "60-learnings", "anvil.bar.md"),
		"type: learning\nid: anvil.bar\nproject: anvil\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	// anvil.src should have two body-relation outgoing edges.
	rows, err := db.LinksFrom("anvil.src")
	if err != nil {
		t.Fatalf("LinksFrom: %v", err)
	}
	bodyRows := 0
	for _, r := range rows {
		if r.Relation == "body" {
			bodyRows++
		}
	}
	if bodyRows != 2 {
		t.Fatalf("expected 2 body-relation rows, got %d: %v", bodyRows, rows)
	}

	// LinksTo("anvil.foo") must surface anvil.src.
	inbound, err := db.LinksTo("anvil.foo")
	if err != nil {
		t.Fatalf("LinksTo: %v", err)
	}
	found := false
	for _, r := range inbound {
		if r.Source == "anvil.src" && r.Relation == "body" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected anvil.src with relation=body in LinksTo(anvil.foo), got %v", inbound)
	}
}
