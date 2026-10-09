package index

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCountByType(t *testing.T) {
	db := openTestDB(t)
	for _, r := range []ArtifactRow{
		{ID: "issue.a", Type: "issue", Path: "/a.md"},
		{ID: "issue.b", Type: "issue", Path: "/b.md"},
		{ID: "learning.c", Type: "learning", Path: "/c.md"},
	} {
		if err := db.UpsertArtifact(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.CountByType()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]int{"issue": 2, "learning": 1}, got); diff != "" {
		t.Errorf("CountByType mismatch (-want +got):\n%s", diff)
	}
}

func TestProjects(t *testing.T) {
	db := openTestDB(t)
	for _, r := range []ArtifactRow{
		{ID: "a", Type: "issue", Project: "zeta", Path: "/a.md"},
		{ID: "b", Type: "issue", Project: "alpha", Path: "/b.md"},
		{ID: "c", Type: "learning", Project: "alpha", Path: "/c.md"},
		{ID: "d", Type: "learning", Path: "/d.md"},
	} {
		if err := db.UpsertArtifact(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"alpha", "zeta"}, got); diff != "" {
		t.Errorf("Projects mismatch (-want +got):\n%s", diff)
	}
}

// Warrant: counting rows instead of distinct sources, or ignoring the type, would inflate or leak counts.
func TestBacklinkCounts(t *testing.T) {
	db := openTestDB(t)
	for _, r := range []ArtifactRow{
		{ID: "decision.a", Type: "decision", Path: "/a.md"},
		{ID: "decision.b", Type: "decision", Path: "/b.md"},
		{ID: "issue.c", Type: "issue", Path: "/c.md"},
		{ID: "issue.d", Type: "issue", Path: "/d.md"},
	} {
		if err := db.UpsertArtifact(r); err != nil {
			t.Fatal(err)
		}
	}
	for src, rows := range map[string][]LinkRow{
		"issue.c": {
			{Source: "issue.c", Target: "decision.a", Relation: "related"},
			{Source: "issue.c", Target: "decision.a", Relation: "depends_on"},
		},
		"issue.d":    {{Source: "issue.d", Target: "decision.a", Relation: "related"}},
		"decision.a": {{Source: "decision.a", Target: "issue.c", Relation: "related"}},
	} {
		if err := db.ReplaceLinks(src, rows); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.BacklinkCounts("decision")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]int{"decision.a": 2}, got); diff != "" {
		t.Errorf("BacklinkCounts mismatch (-want +got):\n%s", diff)
	}
}

// Warrant: a tag joined to the wrong type, or unsorted, would show another type's tags or reorder a row's tags.
func TestTagsByType(t *testing.T) {
	db := openTestDB(t)
	for _, r := range []ArtifactRow{
		{ID: "decision.a", Type: "decision", Path: "/a.md"},
		{ID: "issue.c", Type: "issue", Path: "/c.md"},
	} {
		if err := db.UpsertArtifact(r); err != nil {
			t.Fatal(err)
		}
	}
	for id, tags := range map[string][]string{"decision.a": {"z", "a"}, "issue.c": {"x"}} {
		if err := db.ReplaceTags(id, tags); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.TagsByType("decision")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string][]string{"decision.a": {"a", "z"}}, got); diff != "" {
		t.Errorf("TagsByType mismatch (-want +got):\n%s", diff)
	}
}

// Warrant: fails if RecentlyUpdated orders oldest-first, ignores the limit, breaks ties unstably, or lets sessions in.
func TestRecentlyUpdated(t *testing.T) {
	db := openTestDB(t)
	for _, r := range []ArtifactRow{
		{ID: "issue.old", Type: "issue", Path: "/o.md", Updated: "2026-01-01"},
		{ID: "issue.b", Type: "issue", Path: "/b.md", Updated: "2026-10-09"},
		{ID: "learning.a", Type: "learning", Path: "/a.md", Updated: "2026-10-09"},
		{ID: "thread.mid", Type: "thread", Path: "/m.md", Updated: "2026-05-05"},
		{ID: "session.new", Type: "session", Path: "/s.md", Updated: "2026-10-10"},
	} {
		if err := db.UpsertArtifact(r); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.RecentlyUpdated(3)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.ID)
	}
	if diff := cmp.Diff([]string{"issue.b", "learning.a", "thread.mid"}, got); diff != "" {
		t.Errorf("RecentlyUpdated mismatch (-want +got):\n%s", diff)
	}
}
