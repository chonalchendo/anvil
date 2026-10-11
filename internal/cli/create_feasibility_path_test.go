package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerPutsRunDirBinFirst(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o750); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(bin, "fakebuild")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // test fixture must be executable
		t.Fatal(err)
	}

	other := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(other, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "fakebuild"), []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // test fixture must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", other+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := runFeasibilityBlock("command -v fakebuild", dir, 0)
	if r.runErr != nil || r.exit != 0 {
		t.Fatalf("block failed: %+v", r)
	}
	got, err := os.Stat(strings.TrimSpace(r.output))
	if err != nil {
		t.Fatalf("resolved %q: %v", r.output, err)
	}
	want, err := os.Stat(fake)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(got, want) {
		t.Errorf("fakebuild resolved to %q, want %q", r.output, fake)
	}
}
