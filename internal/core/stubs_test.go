package core

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFindStubs(t *testing.T) {
	vault := t.TempDir()
	mustWrite := func(rel string, body string) {
		full := filepath.Join(vault, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Two stubs at vault root — the Obsidian-wikilink shape.
	mustWrite("issue.burgh.foo.md", "")
	mustWrite("plan.anvil.bar.md", "")
	// Real artifact in canonical dir — must NOT be flagged.
	mustWrite("70-issues/anvil.real.md", "---\ntype: issue\n---\nbody\n")
	// Non-type-prefixed root file — out of scope.
	mustWrite("README.md", "hi")
	// Type-prefixed file nested in a non-canonical dir — out of scope for
	// vault-root scan; nested cruft is not the reported friction.
	mustWrite("notes/issue.something.md", "")

	got, err := FindStubs(vault)
	if err != nil {
		t.Fatalf("FindStubs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d stubs, want 2: %+v", len(got), got)
	}
	names := []string{filepath.Base(got[0].Path), filepath.Base(got[1].Path)}
	sort.Strings(names)
	want := []string{"issue.burgh.foo.md", "plan.anvil.bar.md"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("stub[%d]: got %s want %s", i, names[i], want[i])
		}
	}
	for _, s := range got {
		if s.Size != 0 {
			t.Errorf("stub %s: size %d want 0", s.Path, s.Size)
		}
	}
}

func TestFindStubsMissingRoot(t *testing.T) {
	_, err := FindStubs(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing vault root")
	}
}
