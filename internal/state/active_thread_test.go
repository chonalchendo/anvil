package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActiveThread_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANVIL_STATE_DIR", dir)

	// Empty state returns "", nil.
	got, err := ReadActiveThread()
	if err != nil {
		t.Fatalf("ReadActiveThread on empty state: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}

	// Write then read back.
	id := "foo.research-ducklake"
	if err := WriteActiveThread(id); err != nil {
		t.Fatalf("WriteActiveThread: %v", err)
	}
	got, err = ReadActiveThread()
	if err != nil {
		t.Fatalf("ReadActiveThread after write: %v", err)
	}
	if got != id {
		t.Fatalf("expected %q, got %q", id, got)
	}

	// File exists at expected path.
	p := filepath.Join(dir, "active-thread")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected file at %s: %v", p, err)
	}

	// ClearActiveThread removes the file.
	if err := ClearActiveThread(); err != nil {
		t.Fatalf("ClearActiveThread: %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, got err: %v", err)
	}
}
