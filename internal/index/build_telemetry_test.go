package index

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBuildTasksRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if err := db.InsertBuildRun(BuildRun{
		RunID: "run-1", StartedAt: "2026-06-20T10:00:00Z", Project: "anvil",
		Milestone: "anvil.m1", DryRun: false, Tasks: 2,
	}); err != nil {
		t.Fatalf("InsertBuildRun: %v", err)
	}
	want := []BuildTask{
		{
			RunID: "run-1", TaskID: "anvil.a", Wave: 0, Model: "claude-sonnet-4-6", Effort: "medium",
			Outcome: "success", TokensIn: 100, TokensOut: 50, CacheRead: 10, CacheWrite: 5,
			CostUSD: 0.0123, DurationMS: 4000, AgentTimeMS: 3500, VerifyExit: 0,
		},
		{
			RunID: "run-1", TaskID: "anvil.b", Wave: 1, Model: "claude-opus-4-8", Effort: "high",
			Outcome: "failed", TokensIn: 200, TokensOut: 80, VerifyExit: 1,
		},
	}
	if err := db.InsertBuildTasks(want); err != nil {
		t.Fatalf("InsertBuildTasks: %v", err)
	}

	// A different run must not leak into run-1's rows.
	if err := db.InsertBuildRun(BuildRun{RunID: "run-2", StartedAt: "2026-06-20T11:00:00Z", Tasks: 1}); err != nil {
		t.Fatalf("InsertBuildRun run-2: %v", err)
	}
	if err := db.InsertBuildTasks([]BuildTask{{RunID: "run-2", TaskID: "anvil.c", Model: "claude-haiku-4-5", Outcome: "success"}}); err != nil {
		t.Fatalf("InsertBuildTasks run-2: %v", err)
	}

	got, err := db.BuildTasksByRun("run-1")
	if err != nil {
		t.Fatalf("BuildTasksByRun: %v", err)
	}
	// Ordered by wave then task id.
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildTasksByRun(run-1) mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildTasksByRun_UnknownRunIsEmptyNotNil(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	got, err := db.BuildTasksByRun("nope")
	if err != nil {
		t.Fatalf("BuildTasksByRun: %v", err)
	}
	if got == nil {
		t.Error("want non-nil empty slice for unknown run, got nil")
	}
	if len(got) != 0 {
		t.Errorf("want 0 rows for unknown run, got %d", len(got))
	}
}
