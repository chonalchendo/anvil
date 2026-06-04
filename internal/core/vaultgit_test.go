package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupNudge(t *testing.T) {
	tests := []struct {
		name   string
		status VaultGitStatus
		want   string // substring expected, or "" for no nudge
	}{
		{"not a repo", VaultGitStatus{NotRepo: true}, "not under version control"},
		{"dirty no remote", VaultGitStatus{Dirty: 5, HasRemote: false, LastCommit: "2 days ago"}, "5 uncommitted change(s)"},
		{"no remote only", VaultGitStatus{Dirty: 0, HasRemote: false}, "no off-machine backup"},
		{"never committed", VaultGitStatus{Dirty: 3, HasRemote: true}, "last commit never"},
		{"clean and backed up", VaultGitStatus{Dirty: 0, HasRemote: true, LastCommit: "1 hour ago"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.BackupNudge()
			if tt.want == "" {
				if got != "" {
					t.Fatalf("BackupNudge() = %q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("BackupNudge() = %q, want substring %q", got, tt.want)
			}
		})
	}
}

func TestVaultGitState_NotRepo(t *testing.T) {
	st, err := VaultGitState(t.TempDir())
	if err != nil {
		t.Fatalf("VaultGitState: %v", err)
	}
	if !st.NotRepo {
		t.Fatalf("plain dir: NotRepo = false, want true")
	}
}

func TestVaultGitState_DirtyNoRemote(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := VaultGitState(dir)
	if err != nil {
		t.Fatalf("VaultGitState: %v", err)
	}
	if st.NotRepo {
		t.Fatal("git repo reported as NotRepo")
	}
	if st.Dirty != 1 {
		t.Fatalf("Dirty = %d, want 1", st.Dirty)
	}
	if st.HasRemote {
		t.Fatal("HasRemote = true, want false (no remote configured)")
	}
	if st.LastCommit != "" {
		t.Fatalf("LastCommit = %q, want empty (no commits)", st.LastCommit)
	}
}

func TestVaultGitState_HasRemote(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "git@github.com:owner/repo.git")
	st, err := VaultGitState(dir)
	if err != nil {
		t.Fatalf("VaultGitState: %v", err)
	}
	if !st.HasRemote {
		t.Fatal("HasRemote = false, want true")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...) //nolint:gosec // test-only literals, not user input
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
