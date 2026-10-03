package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func TestIndexAfterSaveCreatesAndUpdates(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	v := &core.Vault{Root: vault}

	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "demo.foo.md"),
		FrontMatter: map[string]any{
			"type":    "issue",
			"id":      "demo.foo",
			"project": "demo",
			"status":  "open",
		},
	}
	if err := indexAfterSave(v, a); err != nil {
		t.Fatalf("indexAfterSave: %v", err)
	}

	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	row, err := db.GetArtifact("issue.demo.foo")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if row.Status != "open" {
		t.Fatalf("status: %q", row.Status)
	}
}

func TestIndexAfterSaveBootstrapsOnFirstUse(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	v := &core.Vault{Root: vault}

	a := &core.Artifact{
		Path:        filepath.Join(vault, "70-issues", "x.md"),
		FrontMatter: map[string]any{"type": "issue", "id": "x", "status": "open"},
	}
	if err := indexAfterSave(v, a); err != nil {
		t.Fatalf("first call (bootstrap): %v", err)
	}
}

// A v4-stamped index holds bare-keyed rows; indexForRead must rebuild it even
// though the vault itself is fresh, so the qualified keys appear.
func TestIndexForReadRebuildsOldSchemaIndex(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "20-learnings"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: learning\nid: foo\nstatus: draft\n---\n"
	if err := os.WriteFile(filepath.Join(vault, "20-learnings", "foo.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := index.Open(index.DBPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Reindex(vault); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSchemaVersion(4); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertArtifact(index.ArtifactRow{ID: "foo", Type: "learning", Status: "draft", Path: filepath.Join(vault, "20-learnings", "foo.md")}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	rdb, err := indexForRead(&core.Vault{Root: vault})
	if err != nil {
		t.Fatalf("indexForRead: %v", err)
	}
	defer rdb.Close() //nolint:errcheck // test cleanup
	if v, _ := rdb.GetSchemaVersion(); v != index.SchemaVersion {
		t.Errorf("schema version = %d, want %d", v, index.SchemaVersion)
	}
	if _, err := rdb.GetArtifact("learning.foo"); err != nil {
		t.Errorf("qualified row missing: %v", err)
	}
	if _, err := rdb.GetArtifact("foo"); err == nil {
		t.Error("bare-keyed row survived the rebuild")
	}
}

func TestResolveIndexIDAmbiguousAcrossTypes(t *testing.T) {
	db := openTestIndex(t, []index.ArtifactRow{
		{ID: "learning.foo", Type: "learning", Status: "draft", Path: "/v/a.md"},
		{ID: "thread.foo", Type: "thread", Status: "open", Path: "/v/b.md"},
		{ID: "learning.solo", Type: "learning", Status: "draft", Path: "/v/c.md"},
	}, nil)
	_, err := resolveIndexID(db, "foo")
	if err == nil || !strings.Contains(err.Error(), `ambiguous id "foo": matches learning.foo, thread.foo`) {
		t.Fatalf("err = %v, want ambiguity", err)
	}
	if got, err := resolveIndexID(db, "solo"); err != nil || got != "learning.solo" {
		t.Errorf("solo = %q, %v", got, err)
	}
	if got, err := resolveIndexID(db, "[[learning.foo]]"); err != nil || got != "learning.foo" {
		t.Errorf("qualified = %q, %v", got, err)
	}
}
