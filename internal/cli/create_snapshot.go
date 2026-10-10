package cli

import (
	"fmt"
	"path/filepath"
	"strings"
)

// snapshotResult is what the --update snapshot reports to the envelope.
// Both fields empty means the prior state was already in vault git.
type snapshotResult struct {
	SHA     string // short sha of the snapshot commit
	Warning string // why no snapshot was taken
}

// snapshotArtifact commits the artifact file's prior state to vault git before
// --update overwrites it. The pathspec is that one file, so the user's other
// uncommitted vault files are never swept in.
func snapshotArtifact(root, path, id string) (snapshotResult, error) {
	if out, err := gitOutput(root, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(out) != "true" {
		return snapshotResult{Warning: "vault is not a git repo; prior state not snapshotted"}, nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return snapshotResult{}, fmt.Errorf("snapshot path: %w", err)
	}
	if err := gitRun(root, "--literal-pathspecs", "add", "--", rel); err != nil {
		return snapshotResult{}, fmt.Errorf("snapshot add: %w", err)
	}
	if gitRun(root, "--literal-pathspecs", "diff", "--cached", "--quiet", "--", rel) == nil {
		return snapshotResult{}, nil
	}
	msg := "anvil snapshot: " + id + " before --update"
	if err := gitRun(root, "--literal-pathspecs", "commit", "-m", msg, "--", rel); err != nil {
		return snapshotResult{}, fmt.Errorf("snapshot commit: %w", err)
	}
	sha, err := gitOutput(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return snapshotResult{}, fmt.Errorf("snapshot sha: %w", err)
	}
	return snapshotResult{SHA: strings.TrimSpace(sha)}, nil
}
