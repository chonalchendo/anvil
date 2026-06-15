package index

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEvalRunsRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // test cleanup

	runs := []EvalRunRow{
		{Skill: "s", EvalID: 0, Name: "happy", Pass: true, Cost: 0.01, Duration: 2 * time.Second, Model: "m", Date: "2026-06-15T10:00:00Z"},
		{Skill: "s", EvalID: 1, Name: "neg", Pass: false, Cost: 0.02, Duration: 3 * time.Second, Model: "m", Date: "2026-06-15T11:00:00Z"},
		{Skill: "other", EvalID: 0, Name: "x", Pass: true, Cost: 0.03, Duration: time.Second, Model: "m", Date: "2026-06-15T12:00:00Z"},
	}
	for _, r := range runs {
		if err := db.RecordEvalRun(r); err != nil {
			t.Fatalf("RecordEvalRun: %v", err)
		}
	}

	got, err := db.EvalHistory("s")
	if err != nil {
		t.Fatalf("EvalHistory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("EvalHistory(s) returned %d rows, want 2 (skill-scoped)", len(got))
	}
	// Most recent first: eval 1 (11:00) before eval 0 (10:00).
	if got[0].EvalID != 1 || got[1].EvalID != 0 {
		t.Errorf("rows not ordered most-recent-first: %d then %d", got[0].EvalID, got[1].EvalID)
	}
	if got[0].Duration != 3*time.Second {
		t.Errorf("Duration round-trip = %v, want 3s", got[0].Duration)
	}
	if !got[1].Pass || got[1].Cost != 0.01 {
		t.Errorf("row fields round-tripped wrong: %+v", got[1])
	}
}
