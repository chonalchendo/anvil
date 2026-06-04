package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VaultGitStatus is the vault's version-control posture, surfaced as a
// backup-discipline nudge at session current/handoff and acted on by
// `anvil vault commit`.
type VaultGitStatus struct {
	NotRepo    bool
	Dirty      int
	HasRemote  bool
	LastCommit string // git's relative age (e.g. "2 days ago"); "" when no commits yet
}

func gitIn(dir string, args ...string) (string, error) {
	c := exec.Command("git", args...) //nolint:gosec // G204: args are package-internal literals, never user input
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// VaultGitState inspects the vault root's git posture. A non-git vault returns
// {NotRepo: true} with no error — "no version control" is a state to report,
// not a failure. The check is for the vault root's *own* .git, not git's
// walk-up: a vault nested inside an unrelated parent repo is still unversioned.
func VaultGitState(root string) (VaultGitStatus, error) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return VaultGitStatus{NotRepo: true}, nil //nolint:nilerr // absent .git is the NotRepo state to report, not an error to propagate
	}
	porcelain, err := gitIn(root, "status", "--porcelain")
	if err != nil {
		return VaultGitStatus{}, fmt.Errorf("vault git status: %w", err)
	}
	dirty := 0
	if porcelain != "" {
		dirty = len(strings.Split(porcelain, "\n"))
	}
	_, remoteErr := gitRemoteOrigin(root)
	lastCommit, _ := gitIn(root, "log", "-1", "--format=%cr") // "" when no commits yet
	return VaultGitStatus{
		Dirty:      dirty,
		HasRemote:  remoteErr == nil,
		LastCommit: lastCommit,
	}, nil
}

// BackupNudge returns a one- or two-line warning when the vault is a data-loss
// risk (untracked, uncommitted, or no off-machine remote), or "" when the vault
// has a clean tree and a remote. Callers print it on stderr so stdout stays
// machine-readable.
func (s VaultGitStatus) BackupNudge() string {
	if s.NotRepo {
		return "⚠ vault backup: not under version control — work has no history. `git init` the vault, then `anvil vault commit`."
	}
	var risks []string
	if s.Dirty > 0 {
		risks = append(risks, fmt.Sprintf("%d uncommitted change(s)", s.Dirty))
	}
	if !s.HasRemote {
		risks = append(risks, "no remote (no off-machine backup)")
	}
	if len(risks) == 0 {
		return ""
	}
	last := s.LastCommit
	if last == "" {
		last = "never"
	}
	return fmt.Sprintf("⚠ vault backup: %s; last commit %s.\n  Run `anvil vault commit` to snapshot; add a git remote + push for off-machine backup.", strings.Join(risks, ", "), last)
}
