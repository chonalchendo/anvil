package index

import (
	"errors"
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

func TestListReadySurfacesUnblockedPrereqButExcludesBlockedDependents(t *testing.T) {
	db := seedReadinessFixture(t)
	got, err := db.ListReady("issue", QueryFilters{})
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	// blocker-open is the target of c's depends_on edge; it has no open blockers of
	// its own so it must surface as ready (highest-priority prerequisite work).
	// c depends on blocker-open and is therefore still blocked — must stay excluded.
	want := []string{"a", "b", "blocker-open"}
	if diff := cmp.Diff(want, ids, cmpopts.SortSlices(func(x, y string) bool { return x < y })); diff != "" {
		t.Fatalf("ready ids mismatch (-want +got):\n%s", diff)
	}
	// Pin both halves: prereq present, blocked dependent absent.
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet["blocker-open"] {
		t.Error("blocker-open (unblocked prereq) must be ready")
	}
	if idSet["c"] {
		t.Error("c (blocked dependent) must not be ready")
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

func seedMilestone(t *testing.T, db *DB, milestone string, issueStatuses map[string]string) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertArtifact(ArtifactRow{ID: milestone, Type: "milestone", Status: "open", Path: "/" + milestone + ".md"}))
	for id, status := range issueStatuses {
		must(db.UpsertArtifact(ArtifactRow{ID: id, Type: "issue", Status: status, Path: "/" + id + ".md"}))
		must(db.ReplaceLinks(id, []LinkRow{{Source: id, Target: milestone, Relation: "milestone"}}))
	}
}

func TestMilestoneStatusDerivesDoneFromLinkedIssues(t *testing.T) {
	tests := []struct {
		name     string
		statuses map[string]string
		want     MilestoneStatus
	}{
		{
			name:     "open issues remain -> not done",
			statuses: map[string]string{"i1": "resolved", "i2": "open"},
			want:     MilestoneStatus{Milestone: "m", Resolved: 1, Total: 2, Done: false},
		},
		{
			name:     "all resolved -> done",
			statuses: map[string]string{"i1": "resolved", "i2": "resolved"},
			want:     MilestoneStatus{Milestone: "m", Resolved: 2, Total: 2, Done: true},
		},
		{
			name:     "no linked issues -> not done",
			statuses: nil,
			want:     MilestoneStatus{Milestone: "m", Resolved: 0, Total: 0, Done: false},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			seedMilestone(t, db, "m", tc.statuses)
			got, err := db.MilestoneStatus("m")
			if err != nil {
				t.Fatalf("MilestoneStatus: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMilestoneStatusUnknownMilestoneErrors(t *testing.T) {
	db := openTestDB(t)
	_, err := db.MilestoneStatus("does-not-exist")
	if !errors.Is(err, ErrArtifactNotInIndex) {
		t.Fatalf("want ErrArtifactNotInIndex, got %v", err)
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
