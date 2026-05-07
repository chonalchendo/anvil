package index

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func seedReadinessFixture(t *testing.T) *DB {
	t.Helper()
	db := openTestDB(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertArtifact(ArtifactRow{ID: "a", Type: "issue", Status: "open", Path: "/a.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "b", Type: "issue", Status: "open", Path: "/b.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "c", Type: "issue", Status: "open", Path: "/c.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "blocker-resolved", Type: "issue", Status: "resolved", Path: "/br.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "blocker-open", Type: "issue", Status: "open", Path: "/bo.md"}))

	must(db.ReplaceLinks("a", nil))
	must(db.ReplaceLinks("b", []LinkRow{{Source: "b", Target: "blocker-resolved", Relation: "blocks"}}))
	must(db.ReplaceLinks("c", []LinkRow{{Source: "c", Target: "blocker-open", Relation: "depends_on"}}))
	return db
}

func TestListReadyExcludesIssuesWithOpenBlockers(t *testing.T) {
	db := seedReadinessFixture(t)
	got, err := db.ListReady("issue", QueryFilters{})
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	want := []string{"a", "b"}
	if diff := cmp.Diff(want, ids, cmpopts.SortSlices(func(x, y string) bool { return x < y })); diff != "" {
		t.Fatalf("ready ids mismatch (-want +got):\n%s", diff)
	}
}

func TestListReadyAppliesStatusFilter(t *testing.T) {
	db := seedReadinessFixture(t)
	got, err := db.ListReady("issue", QueryFilters{Status: "in-progress"})
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 in-progress, got %v", got)
	}
}

func TestListOrphansFindsArtifactsWithNoIncomingLinks(t *testing.T) {
	db := openTestDB(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertArtifact(ArtifactRow{ID: "lonely", Type: "issue", Path: "/l.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "popular", Type: "issue", Path: "/p.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "linker", Type: "issue", Path: "/r.md"}))
	must(db.ReplaceLinks("linker", []LinkRow{{Source: "linker", Target: "popular", Relation: "related"}}))

	got, err := db.ListOrphans(QueryFilters{})
	if err != nil {
		t.Fatalf("ListOrphans: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	want := []string{"linker", "lonely"} // popular is referenced by linker
	if diff := cmp.Diff(want, ids, cmpopts.SortSlices(func(x, y string) bool { return x < y })); diff != "" {
		t.Fatalf("orphans mismatch (-want +got):\n%s", diff)
	}
}

func TestLinksFromAndTo(t *testing.T) {
	db := openTestDB(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertArtifact(ArtifactRow{ID: "src", Type: "issue", Path: "/s.md"}))
	must(db.UpsertArtifact(ArtifactRow{ID: "tgt", Type: "issue", Path: "/t.md"}))
	must(db.ReplaceLinks("src", []LinkRow{{Source: "src", Target: "tgt", Relation: "related"}}))

	from, err := db.LinksFrom("src")
	if err != nil {
		t.Fatal(err)
	}
	if len(from) != 1 || from[0].Target != "tgt" {
		t.Fatalf("LinksFrom: %v", from)
	}
	to, err := db.LinksTo("tgt")
	if err != nil {
		t.Fatal(err)
	}
	if len(to) != 1 || to[0].Source != "src" {
		t.Fatalf("LinksTo: %v", to)
	}
}

func TestLinksUnresolved(t *testing.T) {
	db := openTestDB(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertArtifact(ArtifactRow{ID: "src", Type: "issue", Path: "/s.md"}))
	must(db.ReplaceLinks("src", []LinkRow{
		{Source: "src", Target: "missing", Relation: "related"},
		{Source: "src", Target: "src", Relation: "self"},
	}))
	got, err := db.LinksUnresolved()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Target != "missing" {
		t.Fatalf("unresolved: %v", got)
	}
}
