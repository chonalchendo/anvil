package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// snapshotResult is what the --update snapshot reports to the envelope.
// Both fields empty means the prior state was already in vault git.
type snapshotResult struct {
	SHA     string // short sha of the snapshot commit
	Warning string // why no snapshot was taken
}

// snapshotArtifact commits the artifact file's prior state to vault git before
// --update overwrites it. The pathspec is that one file, so the user's other
// uncommitted vault files are never swept in. A snapshot is a safety net, not
// a gate: every failure restores the file's index entry and becomes a warning.
func snapshotArtifact(root, path, ref string) snapshotResult {
	st, err := core.VaultGitState(root)
	if err != nil {
		return snapshotResult{Warning: fmt.Sprintf("prior state not snapshotted: %v", err)}
	}
	if st.NotRepo {
		return snapshotResult{Warning: "vault is not a git repo; prior state not snapshotted"}
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return snapshotResult{Warning: fmt.Sprintf("prior state not snapshotted: %v", err)}
	}
	// Recorded before `git add` so a failed snapshot restores the user's own
	// staged edit, not HEAD's version.
	entry, _ := gitOutput(root, "--literal-pathspecs", "ls-files", "-s", "--", rel)
	sha, err := commitFile(root, rel, ref)
	if err != nil {
		restoreIndexEntry(root, rel, entry)
		return snapshotResult{Warning: fmt.Sprintf("prior state not snapshotted: %v", err)}
	}
	return snapshotResult{SHA: sha}
}

// commitFile returns the new commit's short sha, or "" when rel already
// matches HEAD.
func commitFile(root, rel, ref string) (string, error) {
	if err := gitRun(root, "--literal-pathspecs", "add", "--", rel); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	if gitRun(root, "--literal-pathspecs", "diff", "--cached", "--quiet", "--", rel) == nil {
		return "", nil
	}
	msg := "anvil snapshot: " + ref + " before --update"
	if err := gitRun(root, "--literal-pathspecs", "commit", "-m", msg, "--", rel); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	sha, err := gitOutput(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(sha), nil
}

// restoreIndexEntry puts the path's index entry back to the recorded
// `ls-files -s` line, or drops it when there was none. Best effort: the caller
// already reports the original failure.
func restoreIndexEntry(root, rel, entry string) {
	if entry == "" {
		_ = gitRun(root, "--literal-pathspecs", "rm", "--cached", "-q", "--ignore-unmatch", "--", rel)
		return
	}
	_ = gitRunStdin(root, entry, "update-index", "--index-info")
}
