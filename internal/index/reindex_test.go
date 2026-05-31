package index

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func writeArtifact(t *testing.T, path, fm string) {
	t.Helper()
	writeArtifactBody(t, path, fm, "body")
}

func writeArtifactBody(t *testing.T, path, fm, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	content := "---\n" + fm + "---\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

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
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

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
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

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

// TestIncrementalSkipsUnchangedFiles verifies that a file not modified since
// the last_reindex stamp is not re-processed, while a modified file is.
func TestIncrementalSkipsUnchangedFiles(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "70-issues")
	pathA := filepath.Join(dir, "a.md")
	pathB := filepath.Join(dir, "b.md")

	writeArtifact(t, pathA, "type: issue\nid: a\nstatus: open\n")
	writeArtifact(t, pathB, "type: issue\nid: b\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// Full rebuild on first call (no stamp).
	if _, err := db.Reindex(vault); err != nil {
		t.Fatal(err)
	}

	// Touch only b with a future mtime so the incremental pass sees it as changed.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(pathB, future, future); err != nil {
		t.Fatal(err)
	}

	// Incremental: only b is re-processed; a is unchanged.
	stats, err := db.Reindex(vault)
	if err != nil {
		t.Fatalf("incremental Reindex: %v", err)
	}
	// Both artifacts must still be present.
	if stats.Artifacts != 2 {
		t.Fatalf("artifacts: got %d want 2", stats.Artifacts)
	}
	if _, err := db.GetArtifact("a"); err != nil {
		t.Fatalf("artifact a missing after incremental: %v", err)
	}
	if _, err := db.GetArtifact("b"); err != nil {
		t.Fatalf("artifact b missing after incremental: %v", err)
	}
}

// TestIncrementalDeletePurgesRows verifies that a file removed since the last
// index has its artifact row and link rows purged.
func TestIncrementalDeletePurgesRows(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "70-issues")
	pathA := filepath.Join(dir, "a.md")
	pathB := filepath.Join(dir, "b.md")

	writeArtifact(t, pathA,
		"type: issue\nid: a\nstatus: open\nmilestone: \"[[milestone.m1]]\"\n")
	writeArtifact(t, pathB, "type: issue\nid: b\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if _, err := db.Reindex(vault); err != nil {
		t.Fatal(err)
	}

	// Verify initial state: 2 artifacts, 1 link from a.
	ac, lc, err := db.countArtifactsAndLinks()
	if err != nil {
		t.Fatal(err)
	}
	if ac != 2 || lc != 1 {
		t.Fatalf("initial state: artifacts=%d links=%d, want 2/1", ac, lc)
	}

	// Delete a.md on disk.
	if err := os.Remove(pathA); err != nil {
		t.Fatal(err)
	}

	// Run incremental — a must be purged along with its link.
	stats, err := db.Reindex(vault)
	if err != nil {
		t.Fatalf("incremental Reindex after delete: %v", err)
	}
	if stats.Artifacts != 1 {
		t.Fatalf("artifacts after delete: got %d want 1", stats.Artifacts)
	}
	if stats.Links != 0 {
		t.Fatalf("links after delete: got %d want 0", stats.Links)
	}
	if _, err := db.GetArtifact("a"); err == nil {
		t.Fatal("artifact a should be purged but is still present")
	}
	links, err := db.LinksFrom("a")
	if err != nil {
		t.Fatalf("LinksFrom a: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("links from a should be purged, got %v", links)
	}
}

// TestIncrementalNoStampFallsBackToFull verifies that a missing last_reindex
// stamp triggers a full rebuild, not an empty incremental pass.
func TestIncrementalNoStampFallsBackToFull(t *testing.T) {
	vault := t.TempDir()
	writeArtifact(t, filepath.Join(vault, "70-issues", "a.md"),
		"type: issue\nid: a\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// No stamp set — Reindex must fall back to full rebuild.
	stats, err := db.Reindex(vault)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Artifacts != 1 {
		t.Fatalf("artifacts: got %d want 1 (full fallback should populate)", stats.Artifacts)
	}
	if _, err := db.GetLastReindex(); err != nil {
		t.Fatalf("stamp not set after full fallback: %v", err)
	}
}

// TestIncrementalEqualsFullRebuild verifies the binding invariant: after an
// incremental pass the artifacts and links tables are identical to a full rebuild.
func TestIncrementalEqualsFullRebuild(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "70-issues")
	pathA := filepath.Join(dir, "a.md")
	pathB := filepath.Join(dir, "b.md")

	writeArtifact(t, pathA,
		"type: issue\nid: a\nstatus: open\nmilestone: \"[[milestone.m1]]\"\n")
	writeArtifact(t, pathB, "type: issue\nid: b\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// First full rebuild to set the stamp.
	if _, err := db.ReindexFull(vault); err != nil {
		t.Fatal(err)
	}

	// Modify a with a future mtime.
	future := time.Now().Add(2 * time.Second)
	writeArtifact(t, pathA,
		"type: issue\nid: a\nstatus: in-progress\nmilestone: \"[[milestone.m1]]\"\n")
	if err := os.Chtimes(pathA, future, future); err != nil {
		t.Fatal(err)
	}

	// Incremental pass.
	incStats, err := db.Reindex(vault)
	if err != nil {
		t.Fatalf("incremental Reindex: %v", err)
	}

	// Full rebuild on the same state.
	fullStats, err := db.ReindexFull(vault)
	if err != nil {
		t.Fatalf("ReindexFull: %v", err)
	}

	if incStats.Artifacts != fullStats.Artifacts {
		t.Errorf("artifact count mismatch: incremental=%d full=%d", incStats.Artifacts, fullStats.Artifacts)
	}
	if incStats.Links != fullStats.Links {
		t.Errorf("link count mismatch: incremental=%d full=%d", incStats.Links, fullStats.Links)
	}

	// Verify a's updated status is reflected.
	row, err := db.GetArtifact("a")
	if err != nil {
		t.Fatalf("GetArtifact a: %v", err)
	}
	if row.Status != "in-progress" {
		t.Errorf("a.status: got %q want in-progress", row.Status)
	}
}

// TestIncrementalRenamePreservedMtimeEqualsFull verifies the binding invariant
// against a rename that preserves mtime (`mv` does). The delete loop purges the
// old path; the new path is ModTime ≤ stamp, so it is caught only because it is
// not a known stored path. Regression guard for the dropped-artifact bug.
func TestIncrementalRenamePreservedMtimeEqualsFull(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "70-issues")
	oldPath := filepath.Join(dir, "a.md")
	newPath := filepath.Join(dir, "a-renamed.md")

	// id lives in frontmatter, so it survives the rename (path-independent).
	writeArtifact(t, oldPath,
		"type: issue\nid: a\nstatus: open\nmilestone: \"[[milestone.m1]]\"\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if _, err := db.ReindexFull(vault); err != nil {
		t.Fatal(err)
	}

	// Rename preserving mtime: read the file's current mtime, move it, restore.
	info, err := os.Stat(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	incStats, err := db.Reindex(vault)
	if err != nil {
		t.Fatalf("incremental Reindex after rename: %v", err)
	}
	fullStats, err := db.ReindexFull(vault)
	if err != nil {
		t.Fatalf("ReindexFull: %v", err)
	}
	if incStats.Artifacts != fullStats.Artifacts {
		t.Errorf("artifact count mismatch after rename: incremental=%d full=%d", incStats.Artifacts, fullStats.Artifacts)
	}
	if incStats.Links != fullStats.Links {
		t.Errorf("link count mismatch after rename: incremental=%d full=%d", incStats.Links, fullStats.Links)
	}
	// The artifact must survive at its new path, not be dropped.
	row, err := db.GetArtifact("a")
	if err != nil {
		t.Fatalf("GetArtifact a after rename: %v", err)
	}
	if row.Path != newPath {
		t.Errorf("a.path: got %q want %q", row.Path, newPath)
	}
}

// TestIncrementalUnparseableEditEqualsFull verifies the binding invariant when a
// previously-valid file is edited into unparseable frontmatter: a full rebuild
// omits it, so incremental must drop the prior row rather than leave it stale.
func TestIncrementalUnparseableEditEqualsFull(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "70-issues")
	pathA := filepath.Join(dir, "a.md")

	writeArtifact(t, pathA,
		"type: issue\nid: a\nstatus: open\nmilestone: \"[[milestone.m1]]\"\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if _, err := db.ReindexFull(vault); err != nil {
		t.Fatal(err)
	}

	// Overwrite a with malformed YAML frontmatter and bump mtime so the
	// incremental pass sees it as changed.
	future := time.Now().Add(2 * time.Second)
	bad := "---\ntype: issue\nid: a\n  bad: : indent\n: nope\n---\n\nbody\n"
	if err := os.WriteFile(pathA, []byte(bad), 0o644); err != nil { //nolint:gosec // 0644 is correct for data files
		t.Fatal(err)
	}
	if err := os.Chtimes(pathA, future, future); err != nil {
		t.Fatal(err)
	}

	incStats, err := db.Reindex(vault)
	if err != nil {
		t.Fatalf("incremental Reindex after unparseable edit: %v", err)
	}
	fullStats, err := db.ReindexFull(vault)
	if err != nil {
		t.Fatalf("ReindexFull: %v", err)
	}
	if incStats.Artifacts != fullStats.Artifacts {
		t.Errorf("artifact count mismatch after unparseable edit: incremental=%d full=%d", incStats.Artifacts, fullStats.Artifacts)
	}
	if incStats.Links != fullStats.Links {
		t.Errorf("link count mismatch after unparseable edit: incremental=%d full=%d", incStats.Links, fullStats.Links)
	}
	// The stale row must be gone, matching the full rebuild's omission.
	if _, err := db.GetArtifact("a"); err == nil {
		t.Error("artifact a should be purged after edit into unparseable frontmatter")
	}
}

// snapshotLinks returns every link row as sorted "source|target|relation|anchor"
// strings, for byte-identical comparison between index code paths.
func snapshotLinks(t *testing.T, db *DB) []string {
	t.Helper()
	rows, err := db.sql.Query(`SELECT source||'|'||target||'|'||relation||'|'||anchor FROM links ORDER BY 1`)
	if err != nil {
		t.Fatalf("query links: %v", err)
	}
	defer rows.Close() //nolint:errcheck // close in defer; error not actionable
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan link: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

// TestIncrementalDuplicateIdEqualsFull reconstructs the live-vault condition from
// anvil.0016: two files derive the SAME id (one via frontmatter `id:`, one via
// path stem) in different directories with different links. The artifacts table
// is keyed by id, so the incremental walk perpetually re-extracts whichever
// colliding path is not currently stored — diverging from full's deterministic
// last-writer-wins and toggling the links table every pass. The fix detects the
// collision and falls back to a full rebuild; this guards both invariants the
// count-only sibling tests miss: byte-identical link rows AND stability across
// two consecutive incremental passes (the toggle).
func TestIncrementalDuplicateIdEqualsFull(t *testing.T) {
	vault := t.TempDir()
	// Issue derives id "dup" from its stem and links out to a milestone.
	writeArtifactBody(t, filepath.Join(vault, "70-issues", "dup.md"),
		"type: issue\nstatus: open\nmilestone: \"[[milestone.m1]]\"\n", "see [[issue.other]]")
	// Plan declares id "dup" in frontmatter and self-references, so a full rebuild
	// (which visits 80-plans after 70-issues) lands the plan's self-links.
	writeArtifactBody(t, filepath.Join(vault, "80-plans", "dup.md"),
		"type: plan\nid: dup\nstatus: open\nissue: dup\n", "self ref [[plan.dup]]")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if _, err := db.ReindexFull(vault); err != nil {
		t.Fatal(err)
	}
	wantLinks := snapshotLinks(t, db)

	// Two consecutive incremental passes must each equal the full rebuild — the
	// first proves no divergence, the second proves no per-run toggle.
	for pass := 1; pass <= 2; pass++ {
		if _, err := db.Reindex(vault); err != nil {
			t.Fatalf("incremental pass %d: %v", pass, err)
		}
		if got := snapshotLinks(t, db); !slices.Equal(got, wantLinks) {
			t.Errorf("incremental pass %d links diverge from full:\n got=%v\nwant=%v", pass, got, wantLinks)
		}
	}
}

// TestReindexFullFlagForcesFull verifies that ReindexFull tears down and
// rebuilds even when no files have changed since the stamp.
func TestReindexFullFlagForcesFull(t *testing.T) {
	vault := t.TempDir()
	writeArtifact(t, filepath.Join(vault, "70-issues", "a.md"),
		"type: issue\nid: a\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if _, err := db.ReindexFull(vault); err != nil {
		t.Fatal(err)
	}
	// ReindexFull again — must not error and must still report 1 artifact.
	stats, err := db.ReindexFull(vault)
	if err != nil {
		t.Fatalf("second ReindexFull: %v", err)
	}
	if stats.Artifacts != 1 {
		t.Fatalf("artifacts: got %d want 1", stats.Artifacts)
	}
}
