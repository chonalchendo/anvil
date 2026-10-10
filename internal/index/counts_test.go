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

func TestCountByTypeStatus(t *testing.T) {
	db := openTestDB(t)
	for _, r := range []ArtifactRow{
		{ID: "a", Type: "issue", Status: "open", Project: "p", Path: "/a.md"},
		{ID: "b", Type: "issue", Status: "open", Project: "p", Path: "/b.md"},
		{ID: "c", Type: "issue", Status: "resolved", Project: "p", Path: "/c.md"},
		{ID: "d", Type: "learning", Status: "draft", Project: "p", Path: "/d.md"},
		{ID: "e", Type: "issue", Status: "open", Project: "other", Path: "/e.md"},
	} {
		if err := db.UpsertArtifact(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.CountByTypeStatus("p")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]int{"issue": {"open": 2, "resolved": 1}, "learning": {"draft": 1}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CountByTypeStatus mismatch (-want +got):\n%s", diff)
	}
}
