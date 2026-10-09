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
