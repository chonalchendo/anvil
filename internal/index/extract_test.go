package index

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestArtifactRowFromFrontmatter(t *testing.T) {
	fm := map[string]any{
		"type":    "issue",
		"id":      "demo.foo",
		"project": "demo",
		"status":  "open",
		"created": "2026-05-07",
		"updated": "2026-05-07",
	}
	got, err := ArtifactRowFromFrontmatter(fm, "/v/70-issues/demo.foo.md")
	if err != nil {
		t.Fatalf("ArtifactRowFromFrontmatter: %v", err)
	}
	want := ArtifactRow{
		ID: "demo.foo", Type: "issue", Status: "open",
		Project: "demo", Path: "/v/70-issues/demo.foo.md",
		Created: "2026-05-07", Updated: "2026-05-07",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("row mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_Scalar(t *testing.T) {
	fm := map[string]any{
		"type":      "issue",
		"id":        "demo.foo",
		"milestone": "[[milestone.demo.m1]]",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	want := []LinkRow{{Source: "demo.foo", Target: "demo.m1", Relation: "milestone", Anchor: ""}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_Array(t *testing.T) {
	fm := map[string]any{
		"type": "decision",
		"id":   "d1",
		"supersedes": []any{
			"[[decision.d0]]",
			"[[decision.d-1]]",
		},
	}
	got := LinkRowsFromFrontmatter("d1", fm)
	want := []LinkRow{
		// "d-1" < "d0" lexicographically ('-' < '0')
		{Source: "d1", Target: "d-1", Relation: "supersedes", Anchor: ""},
		{Source: "d1", Target: "d0", Relation: "supersedes", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_IgnoresNonWikilinks(t *testing.T) {
	fm := map[string]any{
		"type":         "issue",
		"id":           "demo.foo",
		"external_url": "https://example.com",
		"severity":     "high",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows, got %v", got)
	}
}
