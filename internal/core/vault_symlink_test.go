package core

import (
	"os"
	"path/filepath"
	"testing"
)

// filepath.WalkDir will not descend a symlinked root, so a vault reached by a
// link indexes as empty and `anvil rename` rewrites no wikilinks — both silently.
// Resolution has to happen here so every walker inherits a real path.
func TestResolveVaultCanonicalisesSymlinkedRoot(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	if err := os.MkdirAll(store, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(store, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANVIL_VAULT", link)

	v, err := ResolveVault()
	if err != nil {
		t.Fatalf("ResolveVault: %v", err)
	}
	want, err := filepath.EvalSymlinks(store)
	if err != nil {
		t.Fatal(err)
	}
	if v.Root != want {
		t.Errorf("Root = %q, want the link target %q", v.Root, want)
	}
}

// `anvil init` scaffolds a vault that does not exist yet, so an unresolvable
// path survives verbatim rather than being dropped.
func TestResolveVaultKeepsPathThatDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-created-yet")
	t.Setenv("ANVIL_VAULT", missing)

	v, err := ResolveVault()
	if err != nil {
		t.Fatalf("ResolveVault: %v", err)
	}
	if v.Root != missing {
		t.Errorf("Root = %q, want %q", v.Root, missing)
	}
}
