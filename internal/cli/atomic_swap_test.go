package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// setupSwap writes an "old" file with the given content and returns the
// oldPath / newPath pair plus the bytes the swap will install at newPath.
func setupSwap(t *testing.T) (oldPath, newPath string, newContent []byte) {
	t.Helper()
	dir := t.TempDir()
	oldPath = filepath.Join(dir, "old.md")
	newPath = filepath.Join(dir, "new.md")
	if err := os.WriteFile(oldPath, []byte("OLD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return oldPath, newPath, []byte("NEW\n")
}

// assertExactlyOne checks that exactly one of {oldPath, newPath} exists and
// none of the intermediate artefacts (.tmp / .bak) is left on disk.
func assertExactlyOne(t *testing.T, want, gone, tmp, bak string) {
	t.Helper()
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s to exist: %v", want, err)
	}
	if _, err := os.Stat(gone); err == nil {
		t.Errorf("unexpected file %s present (duplicate)", gone)
	}
	if _, err := os.Stat(tmp); err == nil {
		t.Errorf("temp file %s leaked", tmp)
	}
	if _, err := os.Stat(bak); err == nil {
		t.Errorf("backup file %s leaked", bak)
	}
}

func TestAtomicSwap_HappyPath(t *testing.T) {
	oldPath, newPath, content := setupSwap(t)

	if err := atomicSwap(oldPath, newPath, content); err != nil {
		t.Fatalf("swap: %v", err)
	}

	assertExactlyOne(t, newPath, oldPath, newPath+".tmp", oldPath+".bak")

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "NEW\n" {
		t.Errorf("new content = %q, want %q", got, "NEW\n")
	}
}

func TestAtomicSwap_FailAfterTempWrite_OldIntact(t *testing.T) {
	oldPath, newPath, content := setupSwap(t)

	injected := errors.New("injected: after temp write")
	swapHook = &swapFault{afterTempWrite: injected}
	t.Cleanup(func() { swapHook = nil })

	err := atomicSwap(oldPath, newPath, content)
	if !errors.Is(err, injected) {
		t.Fatalf("got err %v, want %v", err, injected)
	}

	// Old must still be intact, new must not exist, no intermediates.
	assertExactlyOne(t, oldPath, newPath, newPath+".tmp", oldPath+".bak")
	got, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "OLD\n" {
		t.Errorf("old content corrupted: %q", got)
	}
}

func TestAtomicSwap_FailAfterOldToBak_RollsBack(t *testing.T) {
	oldPath, newPath, content := setupSwap(t)

	injected := errors.New("injected: after old→bak")
	swapHook = &swapFault{afterOldToBak: injected}
	t.Cleanup(func() { swapHook = nil })

	err := atomicSwap(oldPath, newPath, content)
	if !errors.Is(err, injected) {
		t.Fatalf("got err %v, want %v", err, injected)
	}

	// Rollback should have restored oldPath; new should not exist; no intermediates.
	assertExactlyOne(t, oldPath, newPath, newPath+".tmp", oldPath+".bak")
	got, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "OLD\n" {
		t.Errorf("old content corrupted: %q", got)
	}
}

func TestAtomicSwap_FailAfterTempToNew_NewStateCorrect(t *testing.T) {
	oldPath, newPath, content := setupSwap(t)

	injected := errors.New("injected: after temp→new")
	swapHook = &swapFault{afterTempToNew: injected}
	t.Cleanup(func() { swapHook = nil })

	err := atomicSwap(oldPath, newPath, content)
	if !errors.Is(err, injected) {
		t.Fatalf("got err %v, want %v", err, injected)
	}

	// New must be in place with the new content; oldPath gone.
	if _, serr := os.Stat(oldPath); serr == nil {
		t.Errorf("oldPath %s should not exist after temp→new", oldPath)
	}
	got, rerr := os.ReadFile(newPath)
	if rerr != nil {
		t.Fatalf("newPath missing: %v", rerr)
	}
	if string(got) != "NEW\n" {
		t.Errorf("new content = %q, want %q", got, "NEW\n")
	}
	// .tmp should be gone (renamed). .bak is allowed to linger; this is the
	// step where the swap is committed and remove-bak hadn't run yet.
	if _, serr := os.Stat(newPath + ".tmp"); serr == nil {
		t.Errorf("temp file leaked")
	}
}

func TestAtomicSwap_OldMissing_FailsCleanly(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.md") // never created
	newPath := filepath.Join(dir, "new.md")

	err := atomicSwap(oldPath, newPath, []byte("NEW\n"))
	if err == nil {
		t.Fatal("expected error renaming non-existent old")
	}
	// Temp must not be left behind.
	if _, serr := os.Stat(newPath + ".tmp"); serr == nil {
		t.Errorf("temp file leaked after failed swap")
	}
	if _, serr := os.Stat(newPath); serr == nil {
		t.Errorf("newPath should not exist after failed swap")
	}
}
