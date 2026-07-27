package index

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetAndGetLastReindex(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	if err := db.SetLastReindex(when); err != nil {
		t.Fatalf("SetLastReindex: %v", err)
	}
	got, err := db.GetLastReindex()
	if err != nil {
		t.Fatalf("GetLastReindex: %v", err)
	}
	if !got.Equal(when) {
		t.Fatalf("time mismatch: got %v want %v", got, when)
	}
}

func TestGetLastReindexUnsetReturnsErrUnset(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.GetLastReindex(); !errors.Is(err, ErrLastReindexUnset) {
		t.Fatalf("expected ErrLastReindexUnset, got %v", err)
	}
}

func TestCheckFreshnessReturnsErrIndexStaleWhenVaultNewer(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	if err := db.SetLastReindex(time.Now().Add(-1 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "touch.md"), []byte("x"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(vault, now, now); err != nil {
		t.Fatal(err)
	}

	err = db.CheckFreshness(vault)
	if !errors.Is(err, ErrIndexStale) {
		t.Fatalf("expected ErrIndexStale, got %v", err)
	}
}

func TestCheckFreshnessOKWhenDBNewer(t *testing.T) {
	vault := t.TempDir()
	old := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(vault, old, old); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if err := db.SetLastReindex(time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := db.CheckFreshness(vault); err != nil {
		t.Fatalf("CheckFreshness: %v", err)
	}
}

// In-place file edits don't bump the parent directory's mtime on APFS/ext4,
// so the freshness check has to inspect file mtimes directly.
func TestCheckFreshnessReturnsErrIndexStaleWhenExistingFileEdited(t *testing.T) {
	vault := t.TempDir()
	old := time.Now().Add(-1 * time.Hour)
	if err := os.WriteFile(filepath.Join(vault, "a.md"), []byte("v1"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(vault, "a.md"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(vault, old, old); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if err := db.SetLastReindex(time.Now().Add(-30 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Edit the existing file (content change) without touching the vault dir
	// directly, then explicitly hold the dir mtime steady to simulate the
	// APFS/ext4 behaviour where a content-only edit doesn't propagate.
	if err := os.WriteFile(filepath.Join(vault, "a.md"), []byte("v2 longer content"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	if err := os.Chtimes(vault, old, old); err != nil {
		t.Fatal(err)
	}

	err = db.CheckFreshness(vault)
	if !errors.Is(err, ErrIndexStale) {
		t.Fatalf("expected ErrIndexStale on in-place edit, got %v", err)
	}
}

// A future mtime must not read as drift — see skipFutureMtime.
func TestCheckFreshnessIgnoresFutureMtimeFile(t *testing.T) {
	vault := t.TempDir()
	future := time.Now().Add(24 * time.Hour)
	path := filepath.Join(vault, "x.md")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// Stamp last, exactly as `anvil reindex` does: every filesystem mutation
	// above already happened, so a correct check has nothing left to flag.
	if err := db.SetLastReindex(time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := db.CheckFreshness(vault); err != nil {
		t.Fatalf("future mtime must not be drift, got %v", err)
	}
}

// The root arm is the opposite case: callers signal external drift by stamping
// the vault root slightly ahead of now, so a future root mtime must still read
// as stale or drift absorption stops working.
func TestCheckFreshnessFutureVaultDirMtimeIsStale(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if err := db.SetLastReindex(time.Now()); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(vault, future, future); err != nil {
		t.Fatal(err)
	}

	if err := db.CheckFreshness(vault); !errors.Is(err, ErrIndexStale) {
		t.Fatalf("expected ErrIndexStale on future vault dir mtime, got %v", err)
	}
}

// A deleted vault file must be caught directly by comparing the indexed path
// set against what's on disk — not inferred from the vault directory's mtime,
// which is a signal indistinguishable from any other clock anomaly.
func TestCheckFreshnessDetectsDeletedFileDirectly(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "gone.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	if err := db.UpsertArtifact(ArtifactRow{ID: "demo.gone", Type: "issue", Status: "open", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetLastReindex(time.Now()); err != nil {
		t.Fatal(err)
	}

	// Remove the file without bumping the vault directory's mtime, so the
	// directory-mtime arm alone would miss it.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(vault, old, old); err != nil {
		t.Fatal(err)
	}

	err = db.CheckFreshness(vault)
	if !errors.Is(err, ErrIndexStale) {
		t.Fatalf("expected ErrIndexStale on deleted file, got %v", err)
	}
	if !strings.Contains(err.Error(), "gone.md") {
		t.Fatalf("stale error must name the deleted path, got %q", err)
	}
}

// Reading a vault through a symlinked ancestor (the macOS /tmp →
// /private/tmp case) must not report its live files as deleted: the walked
// paths differ from the indexed ones by symlink resolution alone.
func TestCheckFreshnessSymlinkedVaultRootIsNotStale(t *testing.T) {
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	vault := filepath.Join(realParent, "vault")
	if err := os.MkdirAll(vault, 0o750); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(base, "link")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	path := filepath.Join(vault, "live.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	db, err := Open(filepath.Join(vault, ".anvil", "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	// Indexed under the resolved path, as a prior reindex would have stored it.
	if err := db.UpsertArtifact(ArtifactRow{ID: "demo.live", Type: "issue", Status: "open", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetLastReindex(time.Now()); err != nil {
		t.Fatal(err)
	}

	// Same vault, reached through the symlinked ancestor.
	if err := db.CheckFreshness(filepath.Join(linkParent, "vault")); err != nil {
		t.Fatalf("symlinked vault root must not read as stale, got %v", err)
	}
}

// The stale error has to name the offending file, or the operator can't find
// what to reindex around.
func TestCheckFreshnessNamesStaleFile(t *testing.T) {
	vault := t.TempDir()
	old := time.Now().Add(-1 * time.Hour)
	path := filepath.Join(vault, "drifted.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(vault, ".anvil", "vault.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable
	// Stamp BEFORE the triggering write, so the write is unambiguously the
	// only thing the check can be reacting to.
	if err := db.SetLastReindex(time.Now().Add(-30 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("v2 external edit"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	if err := os.Chtimes(vault, old, old); err != nil {
		t.Fatal(err)
	}

	err = db.CheckFreshness(vault)
	if !errors.Is(err, ErrIndexStale) {
		t.Fatalf("expected ErrIndexStale, got %v", err)
	}
	if !strings.Contains(err.Error(), "drifted.md") {
		t.Fatalf("stale error must name the file, got %q", err)
	}
}
