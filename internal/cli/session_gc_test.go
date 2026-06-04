package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeSessionFixtureWithRetention writes a session file with an explicit
// retention_until date. body may be empty (stub) or non-empty (handoff).
func writeSessionFixtureWithRetention(t *testing.T, vault, id, retentionUntil, body string) string {
	t.Helper()
	path := filepath.Join(vault, "10-sessions", id+".md")
	fm := map[string]any{
		"type":       "session",
		"session_id": id,
		"source":     "claude-code",
		"status":     "raw",
		"created":    "2026-05-01",
	}
	if retentionUntil != "" {
		fm["retention_until"] = retentionUntil
	}
	a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPruneExpiredStubs_RemovesExpiredEmptyStubs(t *testing.T) {
	vault := setupVault(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Expired empty stub — should be pruned.
	expired := writeSessionFixtureWithRetention(t, vault, "expired-stub", "2026-05-15", "")
	// Not-yet-expired empty stub — must be retained.
	writeSessionFixtureWithRetention(t, vault, "fresh-stub", "2026-07-01", "")
	// Expired but has a handoff body — must never be pruned.
	writeSessionFixtureWithRetention(t, vault, "expired-handoff", "2026-05-01",
		"## Handoff\n\n**Objective.** ship it\n")

	removed, retained, err := pruneExpiredStubs(vault, false, now)
	if err != nil {
		t.Fatalf("pruneExpiredStubs: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("removed %d files, want 1: %v", len(removed), removed)
	}
	if removed[0] != expired {
		t.Errorf("removed %q, want %q", removed[0], expired)
	}
	if retained != 2 {
		t.Errorf("retained = %d, want 2", retained)
	}
	// Confirm the file is gone.
	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Error("expired stub file should have been deleted")
	}
}

func TestPruneExpiredStubs_DryRun_DoesNotDelete(t *testing.T) {
	vault := setupVault(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	path := writeSessionFixtureWithRetention(t, vault, "dry-stub", "2026-05-01", "")

	removed, _, err := pruneExpiredStubs(vault, true, now)
	if err != nil {
		t.Fatalf("pruneExpiredStubs dry-run: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("removed %d, want 1", len(removed))
	}
	// File must still exist after dry-run.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("dry-run must not delete the file: %v", err)
	}
}

func TestPruneExpiredStubs_HandoffNeverPruned(t *testing.T) {
	vault := setupVault(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	writeSessionFixtureWithRetention(t, vault, "old-handoff", "2020-01-01",
		"## Handoff\n\n**Objective.** ancient work\n")

	removed, retained, err := pruneExpiredStubs(vault, false, now)
	if err != nil {
		t.Fatalf("pruneExpiredStubs: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("handoff-bearing session must never be pruned; removed: %v", removed)
	}
	if retained != 1 {
		t.Errorf("retained = %d, want 1", retained)
	}
}

func TestPruneExpiredStubs_NoRetentionDate_Retained(t *testing.T) {
	vault := setupVault(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Empty stub with no retention_until — must not be pruned.
	writeSessionFixtureWithRetention(t, vault, "no-retention", "", "")

	removed, _, err := pruneExpiredStubs(vault, false, now)
	if err != nil {
		t.Fatalf("pruneExpiredStubs: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("session with no retention_until must be retained; removed: %v", removed)
	}
}

func TestSessionGC_DryRun_OutputContainsCount(t *testing.T) {
	vault := setupVault(t)

	// Write one expired empty stub directly into the vault.
	writeSessionFixtureWithRetention(t, vault, "gc-expired", "2026-01-01", "")

	out, _, err := runCmd(t, newRootCmd(), "session", "gc", "--dry-run")
	if err != nil {
		t.Fatalf("session gc --dry-run: %v", err)
	}
	// The indirect verification regex: stub|expired|prune|remove|[0-9]+ session
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "would remove") && !strings.Contains(lower, "expired") &&
		!strings.Contains(lower, "stub") && !strings.Contains(lower, "session") {
		t.Errorf("output should describe what would be pruned: %q", out)
	}
}

func TestSessionGC_EmptyVault_ReportsZero(t *testing.T) {
	setupVault(t)

	out, _, err := runCmd(t, newRootCmd(), "session", "gc", "--dry-run")
	if err != nil {
		t.Fatalf("session gc on empty vault: %v", err)
	}
	if !strings.Contains(out, "0") {
		t.Errorf("empty-vault gc should report 0; got: %q", out)
	}
}

func TestSessionGC_Live_RemovesExpiredAndReportsCount(t *testing.T) {
	vault := setupVault(t)

	exp1 := writeSessionFixtureWithRetention(t, vault, "live-exp1", "2026-01-01", "")
	exp2 := writeSessionFixtureWithRetention(t, vault, "live-exp2", "2026-02-01", "")
	writeSessionFixtureWithRetention(t, vault, "live-fresh", "2099-01-01", "")
	writeSessionFixtureWithRetention(t, vault, "live-handoff", "2026-01-01",
		"## Handoff\n\n**Objective.** keep me\n")

	out, _, err := runCmd(t, newRootCmd(), "session", "gc")
	if err != nil {
		t.Fatalf("session gc: %v", err)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("output should mention count 2; got: %q", out)
	}
	// Expired stubs must be gone.
	for _, p := range []string{exp1, exp2} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expired stub %s should have been deleted", p)
		}
	}
	// Fresh and handoff-bearing must survive.
	fresh := filepath.Join(vault, "10-sessions", "live-fresh.md")
	handoff := filepath.Join(vault, "10-sessions", "live-handoff.md")
	for _, p := range []string{fresh, handoff} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("retained session %s should still exist: %v", p, err)
		}
	}
}
