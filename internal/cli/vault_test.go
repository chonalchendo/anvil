package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func seedVaultRepo(t *testing.T, sessionID string) string {
	t.Helper()
	vault := setupVault(t)
	t.Setenv(envSessionID, sessionID)
	t.Setenv("CODEX_HOME", t.TempDir())
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
	return vault
}

func writeVaultFile(t *testing.T, vault, rel, body string) {
	t.Helper()
	p := filepath.Join(vault, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func committedFiles(t *testing.T, vault string) []string {
	t.Helper()
	out, err := gitOutput(vault, "show", "--name-only", "--format=", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(out)
	slices.Sort(got)
	return got
}

func TestVaultCommit_Scope(t *testing.T) {
	const peerIssue = "---\nstatus: in-progress\nclaim_session: peer\n---\n"
	const ownIssue = "---\nstatus: in-progress\nclaim_session: mine\n---\n"
	tests := []struct {
		name     string
		session  string
		setup    func(t *testing.T, vault string)
		want     []string
		wantHeld []string
	}{
		{
			name:    "peer session stub held back",
			session: "mine",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "10-sessions/mine.md", "own")
				writeVaultFile(t, v, "10-sessions/peer.md", "HALF-WRIT")
				writeVaultFile(t, v, "note.md", "x")
			},
			want:     []string{"10-sessions/mine.md", "note.md"},
			wantHeld: []string{"10-sessions/peer.md"},
		},
		{
			name:    "peer-claimed issue held back, own-claimed committed",
			session: "mine",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "70-issues/peer.md", peerIssue)
				writeVaultFile(t, v, "70-issues/own.md", ownIssue)
			},
			want:     []string{"70-issues/own.md"},
			wantHeld: []string{"70-issues/peer.md"},
		},
		{
			name:    "staged-only change committed",
			session: "mine",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "note.md", "x")
				if err := gitRun(v, "add", "note.md"); err != nil {
					t.Fatal(err)
				}
			},
			want: []string{"note.md"},
		},
		{
			name:    "deleted peer stub committed",
			session: "mine",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "10-sessions/dead.md", "old")
				if err := gitRun(v, "add", "-A"); err != nil {
					t.Fatal(err)
				}
				if err := gitRun(v, "commit", "-qm", "stub"); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(v, "10-sessions/dead.md")); err != nil {
					t.Fatal(err)
				}
			},
			want: []string{"10-sessions/dead.md"},
		},
		{
			name:    "unset session id holds every session file",
			session: "",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "10-sessions/live.md", "in-flight")
				writeVaultFile(t, v, "note.md", "x")
			},
			want:     []string{"note.md"},
			wantHeld: []string{"10-sessions/live.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := seedVaultRepo(t, tt.session)
			tt.setup(t, vault)
			_, errOut, err := runCmd(t, newRootCmd(), "vault", "commit", "-m", "snap")
			if err != nil {
				t.Fatalf("vault commit: %v", err)
			}
			for _, h := range tt.wantHeld {
				if !strings.Contains(errOut, h) {
					t.Errorf("stderr should name held file %q: %q", h, errOut)
				}
			}
			if got := committedFiles(t, vault); !slices.Equal(got, tt.want) {
				t.Errorf("committed %v, want %v", got, tt.want)
			}
		})
	}
}
