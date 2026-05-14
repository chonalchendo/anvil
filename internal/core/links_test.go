package core

import (
	"os"
	"path/filepath"
	"testing"
)

func mustWriteIssue(t *testing.T, v *Vault, id string) {
	t.Helper()
	path := filepath.Join(v.Root, "70-issues", id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: issue\ntitle: x\ncreated: 2026-04-29\nstatus: open\nproject: anvil\nseverity: low\n---\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAppendLink_DefaultsToRelated(t *testing.T) {
	v := newScaffolded(t)
	mustWriteIssue(t, v, "anvil.x")
	if err := AppendLink(v, TypeIssue, "anvil.x", TypeLearning, "anvil.gotcha"); err != nil {
		t.Fatal(err)
	}
	a, _ := LoadArtifact(filepath.Join(v.Root, "70-issues", "anvil.x.md"))
	rel, _ := a.FrontMatter["related"].([]any)
	if len(rel) != 1 || rel[0] != "[[learning.anvil.gotcha]]" {
		t.Errorf("expected one related link, got %v", rel)
	}
}

func TestAppendLink_Idempotent(t *testing.T) {
	v := newScaffolded(t)
	mustWriteIssue(t, v, "anvil.x")
	for i := 0; i < 2; i++ {
		if err := AppendLink(v, TypeIssue, "anvil.x", TypeLearning, "anvil.gotcha"); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := LoadArtifact(filepath.Join(v.Root, "70-issues", "anvil.x.md"))
	rel := a.FrontMatter["related"].([]any)
	if len(rel) != 1 {
		t.Errorf("expected idempotent (1 entry), got %d: %v", len(rel), rel)
	}
}

func TestAppendLink_MissingSource_Errors(t *testing.T) {
	v := newScaffolded(t)
	if err := AppendLink(v, TypeIssue, "ghost", TypeLearning, "anvil.gotcha"); err == nil {
		t.Error("expected error for missing source")
	}
}

func TestAppendExternalLink_AppendsAndDedupes(t *testing.T) {
	v := newScaffolded(t)
	mustWriteIssue(t, v, "anvil.x")
	uri := "https://github.com/chonalchendo/anvil/pull/13"
	for i := 0; i < 2; i++ {
		if err := AppendExternalLink(v, TypeIssue, "anvil.x", uri); err != nil {
			t.Fatalf("AppendExternalLink iter %d: %v", i, err)
		}
	}
	a, err := LoadArtifact(filepath.Join(v.Root, "70-issues", "anvil.x.md"))
	if err != nil {
		t.Fatal(err)
	}
	ext, _ := a.FrontMatter["external_links"].([]any)
	if len(ext) != 1 || ext[0] != uri {
		t.Fatalf("external_links = %v, want [%q]", ext, uri)
	}
	if _, ok := a.FrontMatter["related"]; ok {
		t.Fatalf("related should not be touched by AppendExternalLink: %v", a.FrontMatter["related"])
	}
}

func TestAppendExternalLink_MissingSource_Errors(t *testing.T) {
	v := newScaffolded(t)
	if err := AppendExternalLink(v, TypeIssue, "ghost", "https://x"); err == nil {
		t.Error("expected error for missing source")
	}
}
