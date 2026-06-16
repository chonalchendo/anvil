package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLearningTLDRExtraction(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{
			name: "section between headings",
			body: "\n## TL;DR\n\nIndex holds frontmatter only.\n\n## Evidence\n\nverified",
			want: "Index holds frontmatter only.",
		},
		{
			name: "tldr opens body with no blank line",
			body: "## TL;DR\nlone paragraph\n## Caveats\nx",
			want: "lone paragraph",
		},
		{
			name: "tldr runs to end of body",
			body: "## TL;DR\n\nfinal section text",
			want: "final section text",
		},
		{
			name: "absent section",
			body: "## Evidence\n\nno tldr here",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LearningTLDR(c.body); got != c.want {
				t.Fatalf("LearningTLDR: got %q want %q", got, c.want)
			}
		})
	}
}

func TestFTSMatchExpr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"retrieval", `"retrieval"`},
		{"schema rename", `"schema" "rename"`},
		{"  spaced   out ", `"spaced" "out"`},
		{`say "hi"`, `"say" """hi"""`},
		{"   ", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ftsMatchExpr(c.in); got != c.want {
			t.Fatalf("ftsMatchExpr(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

// writeLearning writes a learning artifact with the given id and TL;DR body.
func writeLearning(t *testing.T, vault, id, tldr string) {
	t.Helper()
	writeArtifactBody(t,
		filepath.Join(vault, "20-learnings", id+".md"),
		"type: learning\nid: "+id+"\nstatus: draft\n",
		"## TL;DR\n\n"+tldr+"\n\n## Evidence\n\ne\n\n## Caveats\n\nc")
}

func TestFTSReindexPopulatesAndSearches(t *testing.T) {
	vault := t.TempDir()
	writeLearning(t, vault, "demo.a", "FTS5 makes a learning findable by content not guessed tags")
	writeLearning(t, vault, "demo.b", "Worktree per task lands via pull request review")
	// A non-learning must not enter the FTS table.
	writeArtifact(t, filepath.Join(vault, "70-issues", "demo.iss.md"),
		"type: issue\nid: demo.iss\nstatus: open\n")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	hits, err := db.SearchLearnings("content", QueryFilters{})
	if err != nil {
		t.Fatalf("SearchLearnings: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "demo.a" {
		t.Fatalf("search 'content': got %+v want [demo.a]", hits)
	}

	// Multi-term query is implicit-AND: both terms must be present.
	if hits, _ := db.SearchLearnings("pull review", QueryFilters{}); len(hits) != 1 || hits[0].ID != "demo.b" {
		t.Fatalf("search 'pull review': got %+v want [demo.b]", hits)
	}
	if hits, _ := db.SearchLearnings("content review", QueryFilters{}); len(hits) != 0 {
		t.Fatalf("search 'content review': got %+v want none (no learning has both)", hits)
	}

	// Empty / punctuation-only query yields no terms and no error.
	if hits, err := db.SearchLearnings("   ", QueryFilters{}); err != nil || hits != nil {
		t.Fatalf("blank search: got %+v err %v want nil,nil", hits, err)
	}
}

func TestFTSStaleRowPurgedOnDelete(t *testing.T) {
	vault := t.TempDir()
	writeLearning(t, vault, "demo.a", "findable retrieval content")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if hits, _ := db.SearchLearnings("retrieval", QueryFilters{}); len(hits) != 1 {
		t.Fatalf("pre-delete: want 1 hit, got %d", len(hits))
	}

	if err := db.DeleteArtifact("demo.a"); err != nil {
		t.Fatalf("DeleteArtifact: %v", err)
	}
	if hits, _ := db.SearchLearnings("retrieval", QueryFilters{}); len(hits) != 0 {
		t.Fatalf("post-delete: FTS row not purged, got %d hits", len(hits))
	}
}

// TestFTSSchemaBumpForcesFullBackfill simulates a DB indexed before the FTS
// table existed: last_reindex is stamped (so reindex would go incremental) but
// the FTS table is empty and schema_version lags. A plain incremental Reindex
// must detect the lag, force a full rebuild, and backfill the existing corpus.
func TestFTSSchemaBumpForcesFullBackfill(t *testing.T) {
	vault := t.TempDir()
	writeLearning(t, vault, "demo.a", "findable retrieval content")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	// Roll the DB back to the pre-FTS world: drop indexed content, lag version.
	if _, err := db.sql.Exec(`DELETE FROM learning_fts`); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSchemaVersion(0); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchLearnings("retrieval", QueryFilters{}); len(hits) != 0 {
		t.Fatalf("precondition: FTS should be empty, got %d hits", len(hits))
	}

	// Incremental Reindex must self-heal via a forced full rebuild.
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex after version lag: %v", err)
	}
	if hits, _ := db.SearchLearnings("retrieval", QueryFilters{}); len(hits) != 1 {
		t.Fatalf("backfill: want 1 hit after schema-bump rebuild, got %d", len(hits))
	}
	if v, _ := db.GetSchemaVersion(); v != SchemaVersion {
		t.Fatalf("schema version: got %d want %d", v, SchemaVersion)
	}
}

func TestFTSReindexReflectsEditedTLDR(t *testing.T) {
	vault := t.TempDir()
	writeLearning(t, vault, "demo.a", "original phrasing alpha")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	// Rewrite the TL;DR; a re-extract must drop the old terms and index the new.
	writeLearning(t, vault, "demo.a", "rewritten phrasing beta")
	if _, err := db.ReindexFull(vault); err != nil {
		t.Fatalf("ReindexFull: %v", err)
	}

	if hits, _ := db.SearchLearnings("alpha", QueryFilters{}); len(hits) != 0 {
		t.Fatalf("stale term 'alpha' still matches: %+v", hits)
	}
	if hits, _ := db.SearchLearnings("beta", QueryFilters{}); len(hits) != 1 {
		t.Fatalf("new term 'beta' not indexed: %+v", hits)
	}
}

// TestFTSIncrementalReflectsEditedTLDR covers the production hot path: an
// in-place TL;DR edit picked up by an incremental Reindex (mtime > stamp) must
// re-extract and replace the FTS row, not leave the stale terms.
func TestFTSIncrementalReflectsEditedTLDR(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "20-learnings", "demo.a.md")
	writeLearning(t, vault, "demo.a", "original phrasing alpha")

	db, err := Open(DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	// Edit in place and push mtime past the stamp so the incremental walk
	// re-qualifies the file.
	writeLearning(t, vault, "demo.a", "rewritten phrasing beta")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Reindex(vault); err != nil {
		t.Fatalf("incremental Reindex: %v", err)
	}

	if hits, _ := db.SearchLearnings("alpha", QueryFilters{}); len(hits) != 0 {
		t.Fatalf("stale term 'alpha' still matches after incremental: %+v", hits)
	}
	if hits, _ := db.SearchLearnings("beta", QueryFilters{}); len(hits) != 1 {
		t.Fatalf("new term 'beta' not indexed after incremental: %+v", hits)
	}
}
