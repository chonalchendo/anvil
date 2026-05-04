package core

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeBlankIssue(t *testing.T, v *Vault, id string) {
	t.Helper()
	p := filepath.Join(v.Root, TypeIssue.Dir(), id+".md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("---\ntype: issue\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveLinks_AllPresent(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.x")
	fm := map[string]any{
		"milestone": "[[milestone.anvil.cli-substrate]]",
		"related":   []any{"[[issue.anvil.x]]"},
	}
	mp := filepath.Join(v.Root, TypeMilestone.Dir(), "anvil.cli-substrate.md")
	_ = os.MkdirAll(filepath.Dir(mp), 0o755)
	_ = os.WriteFile(mp, []byte("---\ntype: milestone\n---\n"), 0o644)

	got := ResolveLinks(v, fm)
	if len(got) != 0 {
		t.Errorf("expected 0 unresolved, got %v", got)
	}
}

func TestResolveLinks_DanglingScalar(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{"milestone": "[[milestone.anvil.ghost]]"}
	got := ResolveLinks(v, fm)
	want := []UnresolvedLink{{Field: "milestone", Target: "milestone.anvil.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveLinks_DanglingArrayWithIndex(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.real")
	fm := map[string]any{
		"related": []any{
			"[[issue.anvil.real]]",
			"[[issue.anvil.ghost]]",
		},
	}
	got := ResolveLinks(v, fm)
	want := []UnresolvedLink{{Field: "related[1]", Target: "issue.anvil.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveLinks_NonWikilinkIgnored(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{
		"title":  "Plain string, not a wikilink",
		"status": "open",
	}
	if got := ResolveLinks(v, fm); len(got) != 0 {
		t.Errorf("expected no unresolved, got %v", got)
	}
}

func TestResolveLinks_UnknownTypePrefix_Ignored(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{"author": "[[people.alice]]"}
	if got := ResolveLinks(v, fm); len(got) != 0 {
		t.Errorf("unknown-prefix tokens should be ignored, got %v", got)
	}
}

func TestResolveLinks_Stable(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{
		"milestone": "[[milestone.anvil.ghost]]",
		"related":   []any{"[[issue.anvil.ghost]]"},
	}
	a := ResolveLinks(v, fm)
	b := ResolveLinks(v, fm)
	sort.Slice(a, func(i, j int) bool { return a[i].Field < a[j].Field })
	sort.Slice(b, func(i, j int) bool { return b[i].Field < b[j].Field })
	if !reflect.DeepEqual(a, b) {
		t.Errorf("non-deterministic: %v vs %v", a, b)
	}
}
