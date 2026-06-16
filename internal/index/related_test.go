package index

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// seedRelated populates the DB with a small graph for the related-query tests:
//
//	s  (seed)   tags: domain/cli, activity/issue
//	a  learning tags: domain/cli, activity/issue   -> shares 2
//	b  issue    tags: domain/cli                    -> shares 1
//	c  issue    tags: domain/vault                  -> shares 0 (excluded)
//	d  plan     tags: domain/cli + link s->d        -> shares 1 + link bonus
func seedRelated(t *testing.T, db *DB) {
	t.Helper()
	rows := []struct {
		id, typ string
		tags    []string
	}{
		{"s", "issue", []string{"domain/cli", "activity/issue"}},
		{"a", "learning", []string{"domain/cli", "activity/issue"}},
		{"b", "issue", []string{"domain/cli"}},
		{"c", "issue", []string{"domain/vault"}},
		{"d", "plan", []string{"domain/cli"}},
	}
	for _, r := range rows {
		if err := db.UpsertArtifact(ArtifactRow{ID: r.id, Type: r.typ, Status: "open", Project: "demo", Path: "/p/" + r.id + ".md"}); err != nil {
			t.Fatalf("upsert %s: %v", r.id, err)
		}
		if err := db.ReplaceTags(r.id, r.tags); err != nil {
			t.Fatalf("tags %s: %v", r.id, err)
		}
	}
	if err := db.ReplaceLinks("s", []LinkRow{{Source: "s", Target: "d", Relation: "related"}}); err != nil {
		t.Fatalf("link s->d: %v", err)
	}
}

func TestRelatedByIDRanksAndExcludesSeed(t *testing.T) {
	db := openTestDB(t)
	seedRelated(t, db)

	got, err := db.RelatedByID("s", QueryFilters{})
	if err != nil {
		t.Fatalf("RelatedByID: %v", err)
	}
	want := []RelatedRow{
		{ArtifactRow: ArtifactRow{ID: "d", Type: "plan", Status: "open", Project: "demo", Path: "/p/d.md"}, Score: 3, SharedTags: []string{"domain/cli"}, Links: []string{"related"}},
		{ArtifactRow: ArtifactRow{ID: "a", Type: "learning", Status: "open", Project: "demo", Path: "/p/a.md"}, Score: 2, SharedTags: []string{"activity/issue", "domain/cli"}},
		{ArtifactRow: ArtifactRow{ID: "b", Type: "issue", Status: "open", Project: "demo", Path: "/p/b.md"}, Score: 1, SharedTags: []string{"domain/cli"}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("RelatedByID mismatch (-want +got):\n%s", diff)
	}
}

func TestRelatedByIDProjectFilter(t *testing.T) {
	db := openTestDB(t)
	seedRelated(t, db)

	got, err := db.RelatedByID("s", QueryFilters{Project: "other"})
	if err != nil {
		t.Fatalf("RelatedByID: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("project filter should exclude all demo artifacts, got %d", len(got))
	}
}

func TestRelatedByTagsCountsMatchesIncludingSeed(t *testing.T) {
	db := openTestDB(t)
	seedRelated(t, db)

	got, err := db.RelatedByTags([]string{"domain/cli", "activity/issue"}, QueryFilters{})
	if err != nil {
		t.Fatalf("RelatedByTags: %v", err)
	}
	// No seed artifact, so the tag-bearing artifacts (incl. s) all qualify; no
	// link bonus. Order: Score desc, id asc.
	wantIDs := []string{"a", "s", "b", "d"}
	gotIDs := make([]string, len(got))
	for i, r := range got {
		gotIDs[i] = r.ID
	}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("RelatedByTags order mismatch (-want +got):\n%s", diff)
	}
	if got[0].Score != 2 || got[2].Score != 1 {
		t.Fatalf("scores: a=%d b=%d, want 2 and 1", got[0].Score, got[2].Score)
	}
	if diff := cmp.Diff([]string{"domain/cli"}, got[2].SharedTags); diff != "" {
		t.Fatalf("b SharedTags mismatch (-want +got):\n%s", diff)
	}
}

func TestRelatedByTagsEmptyReturnsNil(t *testing.T) {
	db := openTestDB(t)
	got, err := db.RelatedByTags(nil, QueryFilters{})
	if err != nil || got != nil {
		t.Fatalf("RelatedByTags(nil) = %v, %v; want nil, nil", got, err)
	}
}

func TestReplaceTagsAndDeleteRoundTrip(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertArtifact(ArtifactRow{ID: "x", Type: "issue", Path: "/p/x.md"}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceTags("x", []string{"domain/cli", "domain/cli"}); err != nil { // dup collapses
		t.Fatalf("ReplaceTags: %v", err)
	}
	got, err := db.RelatedByTags([]string{"domain/cli"}, QueryFilters{})
	if err != nil || len(got) != 1 || got[0].ID != "x" {
		t.Fatalf("after ReplaceTags: got %v, err %v", got, err)
	}
	if err := db.DeleteArtifact("x"); err != nil {
		t.Fatalf("DeleteArtifact: %v", err)
	}
	got, err = db.RelatedByTags([]string{"domain/cli"}, QueryFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("DeleteArtifact left tag rows: %v", got)
	}
}

func TestTagsFromFrontmatter(t *testing.T) {
	fm := map[string]any{"tags": []any{"domain/cli", "", 42, "activity/issue"}}
	got := TagsFromFrontmatter(fm)
	want := []string{"domain/cli", "activity/issue"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TagsFromFrontmatter mismatch (-want +got):\n%s", diff)
	}
	if TagsFromFrontmatter(map[string]any{}) != nil {
		t.Fatalf("missing tags should yield nil")
	}
}
