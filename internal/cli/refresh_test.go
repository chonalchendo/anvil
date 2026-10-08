package cli

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/core"
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
		{ID: "learning.l.drifted", Type: "learning", Status: "verified", Path: "/v/l.drifted.md"},
		{ID: "learning.l.fresh", Type: "learning", Status: "verified", Path: "/v/l.fresh.md"},
		{ID: "learning.l.draft-drift", Type: "learning", Status: "draft", Path: "/v/l.draft-drift.md"},
		{ID: "learning.l.already-stale", Type: "learning", Status: "stale", Path: "/v/l.already-stale.md"},
		{ID: "learning.l.retracted", Type: "learning", Status: "retracted", Path: "/v/l.retracted.md"},
		{ID: "issue.anvil.alive", Type: "issue", Status: "open", Path: "/v/anvil.alive.md"},
	}
	links := []index.LinkRow{
		// drifted: one related target gone, one present, plus a body link gone (ignored).
		{Source: "learning.l.drifted", Target: "anvil.gone", Relation: "related"},
		{Source: "learning.l.drifted", Target: "issue.anvil.alive", Relation: "related"},
		{Source: "learning.l.drifted", Target: "anvil.body-gone", Relation: "body"},
		// fresh: only resolvable related targets.
		{Source: "learning.l.fresh", Target: "issue.anvil.alive", Relation: "related"},
		// draft-drift: a missing related target on a draft learning.
		{Source: "learning.l.draft-drift", Target: "anvil.gone", Relation: "related"},
		// already-stale and retracted: missing related, but excluded (no edge to stale).
		{Source: "learning.l.already-stale", Target: "anvil.gone", Relation: "related"},
		{Source: "learning.l.retracted", Target: "anvil.gone", Relation: "related"},
	}
	db := openTestIndex(t, arts, links)

	got, checked, err := staleLearnings(db)
	if err != nil {
		t.Fatalf("staleLearnings: %v", err)
	}
	// Draft and verified learnings are eligible (both have an edge to stale):
	// drifted, fresh, draft-drift. already-stale and retracted are excluded by
	// the state machine even though both have a dead related link.
	if checked != 3 {
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

// TestFreshnessCommandRespectsStateMachine drives the full command through a
// temp vault and asserts the on-disk transition obeys the learning state
// machine: verified and draft learnings with a dead related link go stale, a
// retracted one does not (retracted→stale is not an edge).
func TestFreshnessCommandRespectsStateMachine(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	tags := []string{"--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"}

	execCmd(t, append([]string{"create", "learning", "--title", "drifted verified claim"}, tags...)...)
	execCmd(t, "set", "learning", "drifted-verified-claim", "related", "[[issue.demo.ghost]]")
	execCmd(t, "transition", "learning", "drifted-verified-claim", "verified")

	execCmd(t, append([]string{"create", "learning", "--title", "drifted draft claim"}, tags...)...)
	execCmd(t, "set", "learning", "drifted-draft-claim", "related", "[[issue.demo.ghost]]")

	execCmd(t, append([]string{"create", "learning", "--title", "drifted retracted claim"}, tags...)...)
	execCmd(t, "set", "learning", "drifted-retracted-claim", "related", "[[issue.demo.ghost]]")
	execCmd(t, "transition", "learning", "drifted-retracted-claim", "verified")
	execCmd(t, "transition", "learning", "drifted-retracted-claim", "retracted")

	execCmd(t, "reindex")
	execCmd(t, "refresh", "learnings")

	dir := filepath.Join(vault, core.TypeLearning.Dir())
	for id, want := range map[string]string{
		"drifted-verified-claim":  "stale",     // verified→stale: legal, drifted
		"drifted-draft-claim":     "stale",     // draft→stale: legal, drifted
		"drifted-retracted-claim": "retracted", // retracted→stale: no edge, untouched
	} {
		a, err := core.LoadArtifact(filepath.Join(dir, id+".md"))
		if err != nil {
			t.Fatalf("load %s: %v", id, err)
		}
		if got, _ := a.FrontMatter["status"].(string); got != want {
			t.Errorf("%s status = %q, want %q", id, got, want)
		}
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
