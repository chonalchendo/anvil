package cli

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/index"
)

// openTestIndex returns a DB seeded with the given artifacts and links.
func openTestIndex(t *testing.T, arts []index.ArtifactRow, links []index.LinkRow) *index.DB {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, a := range arts {
		if err := db.UpsertArtifact(a); err != nil {
			t.Fatalf("UpsertArtifact %s: %v", a.ID, err)
		}
	}
	bySource := map[string][]index.LinkRow{}
	for _, l := range links {
		bySource[l.Source] = append(bySource[l.Source], l)
	}
	for src, rows := range bySource {
		if err := db.ReplaceLinks(src, rows); err != nil {
			t.Fatalf("ReplaceLinks %s: %v", src, err)
		}
	}
	return db
}

func TestFreshnessStalesMissingRelated(t *testing.T) {
	arts := []index.ArtifactRow{
		{ID: "l.drifted", Type: "learning", Status: "verified", Path: "/v/l.drifted.md"},
		{ID: "l.fresh", Type: "learning", Status: "verified", Path: "/v/l.fresh.md"},
		{ID: "l.draft-drift", Type: "learning", Status: "draft", Path: "/v/l.draft-drift.md"},
		{ID: "l.already-stale", Type: "learning", Status: "stale", Path: "/v/l.already-stale.md"},
		{ID: "anvil.alive", Type: "issue", Status: "open", Path: "/v/anvil.alive.md"},
	}
	links := []index.LinkRow{
		// drifted: one related target gone, one present, plus a body link gone (ignored).
		{Source: "l.drifted", Target: "anvil.gone", Relation: "related"},
		{Source: "l.drifted", Target: "anvil.alive", Relation: "related"},
		{Source: "l.drifted", Target: "anvil.body-gone", Relation: "body"},
		// fresh: only resolvable related targets.
		{Source: "l.fresh", Target: "anvil.alive", Relation: "related"},
		// draft-drift: a missing related target on a draft learning.
		{Source: "l.draft-drift", Target: "anvil.gone", Relation: "related"},
		// already-stale: missing related, but excluded (not draft/verified).
		{Source: "l.already-stale", Target: "anvil.gone", Relation: "related"},
	}
	db := openTestIndex(t, arts, links)

	got, checked, err := staleLearnings(db)
	if err != nil {
		t.Fatalf("staleLearnings: %v", err)
	}
	if checked != 3 { // drifted + fresh + draft-drift; already-stale not examined
		t.Errorf("checked = %d, want 3", checked)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })
	want := []staleCandidate{
		{ID: "l.draft-drift", Path: "/v/l.draft-drift.md", Missing: []string{"anvil.gone"}},
		{ID: "l.drifted", Path: "/v/l.drifted.md", Missing: []string{"anvil.gone"}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("staleLearnings mismatch (-want +got):\n%s", diff)
	}
}

func TestFreshnessNoLearnings(t *testing.T) {
	db := openTestIndex(t, nil, nil)
	got, checked, err := staleLearnings(db)
	if err != nil {
		t.Fatalf("staleLearnings: %v", err)
	}
	if checked != 0 || len(got) != 0 {
		t.Errorf("want checked=0 and no candidates, got checked=%d candidates=%d", checked, len(got))
	}
}
