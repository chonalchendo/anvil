package index

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExportTraces(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	traces := []Trace{
		{TaskID: "task-1", Prompt: "fix the bug", Outcome: "success", Model: "claude-sonnet-4-6", Effort: "medium"},
		{TaskID: "task-2", Prompt: "add tests", Outcome: "failed", Model: "claude-sonnet-4-6", Effort: "medium"},
		{TaskID: "task-3", Prompt: "refactor auth", Outcome: "success", Model: "claude-sonnet-4-6", Effort: "high"},
	}
	for _, tr := range traces {
		if err := db.InsertTrace(tr); err != nil {
			t.Fatalf("InsertTrace %s: %v", tr.TaskID, err)
		}
	}

	got, err := db.ListSuccessfulTraces()
	if err != nil {
		t.Fatalf("ListSuccessfulTraces: %v", err)
	}

	// Only success rows returned, in insertion order, with auto-assigned IDs.
	want := []Trace{
		{ID: 1, TaskID: "task-1", Prompt: "fix the bug", Outcome: "success", Model: "claude-sonnet-4-6", Effort: "medium"},
		{ID: 3, TaskID: "task-3", Prompt: "refactor auth", Outcome: "success", Model: "claude-sonnet-4-6", Effort: "high"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListSuccessfulTraces mismatch (-want +got):\n%s", diff)
	}
}

func TestExportTracesEmpty(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), ".anvil", "vault.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	got, err := db.ListSuccessfulTraces()
	if err != nil {
		t.Fatalf("ListSuccessfulTraces: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty result, got %d rows", len(got))
	}
}
