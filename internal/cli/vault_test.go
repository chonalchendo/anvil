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
		wantNoop bool // nothing to commit, yet the vault must end clean
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
		{
			name:    "staged rename committed with its origin deletion",
			session: "mine",
			setup: func(t *testing.T, v string) {
				commitSeed(t, v, "seed.md")
				gitMust(t, v, "mv", "seed.md", "moved.md")
			},
			want: []string{"moved.md"}, // --name-only collapses the rename pair
		},
		{
			name:    "staged delete committed",
			session: "mine",
			setup: func(t *testing.T, v string) {
				commitSeed(t, v, "seed.md")
				gitMust(t, v, "rm", "-q", "seed.md")
			},
			want: []string{"seed.md"},
		},
		{
			name:    "staged-new then deleted from disk does not abort",
			session: "mine",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "tmp.md", "x")
				gitMust(t, v, "add", "tmp.md")
				if err := os.Remove(filepath.Join(v, "tmp.md")); err != nil {
					t.Fatal(err)
				}
				writeVaultFile(t, v, "note.md", "x")
			},
			want: []string{"note.md"},
		},
		{
			name:    "intent-to-add after mv commits rename with origin deletion",
			session: "mine",
			setup: func(t *testing.T, v string) {
				commitSeed(t, v, "seed.md")
				if err := os.Rename(filepath.Join(v, "seed.md"), filepath.Join(v, "moved.md")); err != nil {
					t.Fatal(err)
				}
				gitMust(t, v, "add", "-N", "moved.md")
			},
			want: []string{"moved.md"},
		},
		{
			name:    "staged-new deleted from disk alone leaves the vault clean",
			session: "mine",
			setup: func(t *testing.T, v string) {
				writeVaultFile(t, v, "tmp.md", "x")
				gitMust(t, v, "add", "tmp.md")
				if err := os.Remove(filepath.Join(v, "tmp.md")); err != nil {
					t.Fatal(err)
				}
			},
			wantNoop: true,
		},
		{
			name:    "rename into a held path holds both ends",
			session: "mine",
			setup: func(t *testing.T, v string) {
				commitSeed(t, v, "seed.md")
				gitMust(t, v, "mv", "seed.md", "10-sessions/peer.md")
				writeVaultFile(t, v, "note.md", "x")
			},
			want:     []string{"note.md"},
			wantHeld: []string{"10-sessions/peer.md"},
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
			if tt.wantNoop {
				if st, _ := gitOutput(vault, "status", "--porcelain"); st != "" {
					t.Errorf("vault should be clean, got %q", st)
				}
				return
			}
			if got := committedFiles(t, vault); !slices.Equal(got, tt.want) {
				t.Errorf("committed %v, want %v", got, tt.want)
			}
		})
	}
}

func gitMust(t *testing.T, v string, args ...string) {
	t.Helper()
	if err := gitRun(v, args...); err != nil {
		t.Fatal(err)
	}
}

func commitSeed(t *testing.T, v, rel string) {
	t.Helper()
	writeVaultFile(t, v, rel, "seed")
	gitMust(t, v, "add", "-A")
	gitMust(t, v, "commit", "-qm", "add "+rel)
}

func TestSessionEnd_HookPayloadSessionID(t *testing.T) {
	vault := seedVaultRepo(t, "")
	writeVaultFile(t, vault, "10-sessions/mine.md", "own")
	writeVaultFile(t, vault, "10-sessions/peer.md", "HALF-WRIT")
	root := newRootCmd()
	root.SetIn(strings.NewReader(`{"session_id":"mine"}`))
	if _, _, err := runCmd(t, root, "session", "end", "--commit"); err != nil {
		t.Fatal(err)
	}
	if got := committedFiles(t, vault); !slices.Equal(got, []string{"10-sessions/mine.md"}) {
		t.Errorf("committed %v, want only the payload session's stub", got)
	}
}

func seedVaultWithRemote(t *testing.T, sessionID string, upstream bool) (vault, remote string) {
	t.Helper()
	vault = seedVaultRepo(t, sessionID)
	remote = t.TempDir()
	gitMust(t, remote, "init", "-q", "--bare")
	gitMust(t, vault, "remote", "add", "origin", remote)
	if upstream {
		gitMust(t, vault, "push", "-q", "-u", "origin", "HEAD")
	}
	return vault, remote
}

func unpushed(t *testing.T, vault string) string {
	t.Helper()
	out, err := gitOutput(vault, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestSessionEnd_PushesPeerOnlyDirtyWithUnpushedCommit(t *testing.T) {
	vault, _ := seedVaultWithRemote(t, "mine", true)
	writeVaultFile(t, vault, "10-sessions/earlier.md", "own")
	gitMust(t, vault, "add", "-A")
	gitMust(t, vault, "commit", "-qm", "local")
	// Only a peer's in-flight file is dirty: nothing to commit, yet the push must run.
	writeVaultFile(t, vault, "10-sessions/peer.md", "HALF-WRIT")
	if _, _, err := runCmd(t, newRootCmd(), "session", "end", "--commit", "--push"); err != nil {
		t.Fatal(err)
	}
	if got := unpushed(t, vault); got != "0" {
		t.Errorf("unpushed = %s, want 0", got)
	}
}

func TestSessionEnd_CleanVaultNothingUnpushedDoesNotPush(t *testing.T) {
	vault, _ := seedVaultWithRemote(t, "mine", true)
	stdout, _, err := runCmd(t, newRootCmd(), "session", "end", "--commit", "--push")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "pushed") {
		t.Errorf("stdout %q claims a push with nothing unpushed", stdout)
	}
	if got := unpushed(t, vault); got != "0" {
		t.Errorf("unpushed = %s", got)
	}
}

func TestVaultCommit_PushWithoutUpstreamHints(t *testing.T) {
	vault, _ := seedVaultWithRemote(t, "mine", false)
	writeVaultFile(t, vault, "10-sessions/mine.md", "own")
	stdout, _, err := runCmd(t, newRootCmd(), "vault", "commit", "--push")
	if err != nil {
		t.Fatalf("no-upstream push must not fail: %v", err)
	}
	if !strings.Contains(stdout, "git push -u origin") {
		t.Errorf("stdout %q lacks the upstream hint", stdout)
	}
}

func TestVaultCommit_PushesUnpushedOnCleanVaultAndFailsLoud(t *testing.T) {
	vault, remote := seedVaultWithRemote(t, "mine", true)
	writeVaultFile(t, vault, "10-sessions/mine.md", "own")
	gitMust(t, vault, "add", "-A")
	gitMust(t, vault, "commit", "-qm", "local")
	if _, _, err := runCmd(t, newRootCmd(), "vault", "commit", "--push"); err != nil {
		t.Fatal(err)
	}
	if got := unpushed(t, vault); got != "0" {
		t.Errorf("unpushed = %s, want 0", got)
	}
	gitMust(t, vault, "remote", "set-url", "origin", filepath.Join(remote, "missing"))
	writeVaultFile(t, vault, "10-sessions/mine.md", "changed")
	if _, _, err := runCmd(t, newRootCmd(), "session", "end", "--commit", "--push"); err == nil {
		t.Error("unreachable remote returned nil, want error")
	}
}
