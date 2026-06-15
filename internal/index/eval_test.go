package index

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func intp(n int) *int { return &n }

func TestEvalHistoryRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// A grading.json-shaped run (counts present) and a history.json-shaped run
	// (counts nil — the iteration schema omits them).
	want := []EvalRun{
		{Skill: "pdf", Source: "skill-creator", Ref: "v1", Passed: intp(3), Failed: intp(1), Total: intp(4), PassRate: 0.75, Date: "2026-06-15"},
		{Skill: "pdf", Source: "skill-creator", Ref: "v2", PassRate: 0.85, Date: "2026-06-15"},
	}
	for _, r := range want {
		if err := db.InsertEvalRun(r); err != nil {
			t.Fatalf("InsertEvalRun: %v", err)
		}
	}
	// A different skill must not leak into pdf's history.
	if err := db.InsertEvalRun(EvalRun{Skill: "other", Source: "skill-creator", PassRate: 0.5, Date: "2026-06-15"}); err != nil {
		t.Fatalf("InsertEvalRun other: %v", err)
	}

	got, err := db.EvalHistory("pdf")
	if err != nil {
		t.Fatalf("EvalHistory: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("EvalHistory mismatch (-want +got):\n%s", diff)
	}
}

func TestEvalHistoryEmpty(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	got, err := db.EvalHistory("nope")
	if err != nil {
		t.Fatalf("EvalHistory: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty history, got %d rows", len(got))
	}
}
