package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestArtifact_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := &Artifact{
		Path: filepath.Join(dir, "issue.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "created": "2026-04-29", "status": "external",
		},
		Body: "## Context\n\nbody.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := LoadArtifact(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(a.FrontMatter, got.FrontMatter); diff != "" {
		t.Errorf("frontmatter mismatch:\n%s", diff)
	}
	if !strings.Contains(got.Body, "## Context") {
		t.Errorf("body lost: %q", got.Body)
	}
}

func TestLoadArtifact_NoFrontmatter_Errors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "raw.md")
	if err := os.WriteFile(p, []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadArtifact(p); err == nil {
		t.Error("expected error, got nil")
	}
}
