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
		ID: "issue.demo.foo", Type: "issue", Status: "open",
		Project: "demo", Path: "/v/70-issues/demo.foo.md",
		Created: "2026-05-07", Updated: "2026-05-07",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("row mismatch (-want +got):\n%s", diff)
	}
}

// TestArtifactRowFromFrontmatter_PathStemCanonicalised pins the join side of the
// canonical-id rule: a file whose name carries its type prefix registers under
// the same id LinkRowsFrom* targets, so incoming edges to it resolve. A design
// type's stem is bare going forward and prefixed in the back catalogue; both
// must key on the qualified IndexKey.
func TestArtifactRowFromFrontmatter_PathStemCanonicalised(t *testing.T) {
	cases := []struct {
		typ, path, want string
	}{
		{"issue", "/v/70-issues/issue.demo.0001.probe.md", "issue.demo.0001.probe"},
		{"issue", "/v/70-issues/demo.0002.plain.md", "issue.demo.0002.plain"},
		{"system-design", "/v/06-system-designs/system-design.demo.md", "system-design.demo"},
		{"system-design", "/v/06-system-designs/demo.md", "system-design.demo"},
	}
	for _, tc := range cases {
		got, err := ArtifactRowFromFrontmatter(map[string]any{"type": tc.typ}, tc.path)
		if err != nil {
			t.Fatalf("ArtifactRowFromFrontmatter(%s): %v", tc.path, err)
		}
		if got.ID != tc.want {
			t.Errorf("id for %s = %q, want %q", tc.path, got.ID, tc.want)
		}
	}
}

func TestLinkRowsFromFrontmatter_Scalar(t *testing.T) {
	fm := map[string]any{
		"type":      "issue",
		"id":        "demo.foo",
		"milestone": "[[milestone.demo.m1]]",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	want := []LinkRow{{Source: "demo.foo", Target: "milestone.demo.m1", Relation: "milestone", Anchor: ""}}
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
		{Source: "d1", Target: "decision.d-1", Relation: "supersedes", Anchor: ""},
		{Source: "d1", Target: "decision.d0", Relation: "supersedes", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_IgnoresNonWikilinks(t *testing.T) {
	fm := map[string]any{
		"type":           "issue",
		"id":             "demo.foo",
		"external_links": []any{"https://example.com", "abc1234"},
		"severity":       "high",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows, got %v", got)
	}
}
