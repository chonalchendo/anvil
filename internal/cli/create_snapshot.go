package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
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
	out, gitErr := gitOutput(root, "rev-parse", "--is-inside-work-tree")
	if notRepo := gitErr != nil || strings.TrimSpace(out) != "true"; notRepo {
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

// emitUpdatedResult emits the updated envelope with the snapshot sha and any
// skip warning folded in. It wraps emitCreateResult so that function keeps one
// signature for create, already_exists and updated.
func emitUpdatedResult(cmd *cobra.Command, asJSON bool, id, path string, findings []*errfmt.ValidationError, changed []string, snap snapshotResult) error {
	if !asJSON {
		if err := emitCreateResult(cmd, false, id, path, statusUpdated, nil, findings, changed); err != nil {
			return err
		}
		if snap.SHA != "" {
			cmd.Println("snapshot: " + snap.SHA)
		}
		if snap.Warning != "" {
			cmd.PrintErrln("warning: " + snap.Warning)
		}
		return nil
	}
	var buf bytes.Buffer
	orig := cmd.OutOrStdout()
	cmd.SetOut(&buf)
	err := emitCreateResult(cmd, true, id, path, statusUpdated, nil, findings, changed)
	cmd.SetOut(orig)
	if err != nil {
		return err
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		return err
	}
	if snap.SHA != "" {
		payload["snapshot"] = snap.SHA
	}
	if snap.Warning != "" {
		ws, _ := payload["warnings"].([]any)
		payload["warnings"] = append(ws, map[string]string{"kind": "snapshot", "got": snap.Warning})
	}
	out, _ := json.Marshal(payload)
	_, err = io.WriteString(orig, string(out)+"\n")
	return err
}
