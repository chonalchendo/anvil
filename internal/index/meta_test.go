package index

import (
	"errors"
	"os"
	"path/filepath"
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
	defer db.Close()

	if err := db.SetLastReindex(time.Now().Add(-1 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "touch.md"), []byte("x"), 0o644); err != nil {
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
	defer db.Close()
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
	if err := os.WriteFile(filepath.Join(vault, "a.md"), []byte("v1"), 0o644); err != nil {
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
	defer db.Close()
	if err := db.SetLastReindex(time.Now().Add(-30 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Edit the existing file (content change) without touching the vault dir
	// directly, then explicitly hold the dir mtime steady to simulate the
	// APFS/ext4 behaviour where a content-only edit doesn't propagate.
	if err := os.WriteFile(filepath.Join(vault, "a.md"), []byte("v2 longer content"), 0o644); err != nil {
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
