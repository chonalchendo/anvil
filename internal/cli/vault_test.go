package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultCommit_HoldsBackPeerSessionFiles(t *testing.T) {
	vault := setupVault(t)
	t.Setenv(envSessionID, "mine")
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "a@b.c"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "-qm", "seed", "--allow-empty"},
	} {
		if err := gitRun(vault, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	sessions := filepath.Join(vault, "10-sessions")
	if err := os.MkdirAll(sessions, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"mine.md": "own", "peer.md": "HALF-WRIT"} {
		if err := os.WriteFile(filepath.Join(sessions, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "note.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, errOut, err := runCmd(t, newRootCmd(), "vault", "commit", "-m", "snap")
	if err != nil {
		t.Fatalf("vault commit: %v", err)
	}
	if !strings.Contains(errOut, "10-sessions/peer.md") {
		t.Errorf("stderr should name the held-back file: %q", errOut)
	}
	out, err := gitOutput(vault, "show", "--name-only", "--format=", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(out)
	want := map[string]bool{"10-sessions/mine.md": true, "note.md": true}
	if len(got) != len(want) {
		t.Fatalf("committed %v, want %v", got, want)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected committed file %q", f)
		}
	}
}
