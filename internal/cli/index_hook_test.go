package cli

import (
	"path/filepath"
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
	defer db.Close()
	row, err := db.GetArtifact("demo.foo")
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
