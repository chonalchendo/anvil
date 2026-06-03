package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

var validSessionSources = []string{"claude-code", "codex", "chatgpt", "claude-web", "cursor", "continue"}

func runCreateSession(cmd *cobra.Command, v *core.Vault, sessionID, source, startedAt, activeThread string, asJSON, update bool) error {
	if sessionID == "" {
		return fmt.Errorf("--session-id is required for session")
	}
	validSource := false
	for _, s := range validSessionSources {
		if s == source {
			validSource = true
			break
		}
	}
	if !validSource {
		return formatEnumError("--source", source, validSessionSources, "")
	}

	now := time.Now().UTC()
	if startedAt == "" {
		startedAt = now.Format(time.RFC3339)
	}
	path := filepath.Join(v.Root, core.TypeSession.Dir(), sessionID+".md")

	if existing, err := core.LoadArtifact(path); err == nil {
		if !update {
			if drift := sessionDrift(existing.FrontMatter, source, startedAt, activeThread); drift != "" {
				return fmt.Errorf("session %s already exists with different %s; use --update to rewrite", sessionID, drift)
			}
			if asJSON {
				return emitSessionJSON(cmd, sessionID, path, activeThread)
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", path, err)
	}

	if _, err := writeSessionFile(v, path, sessionID, source, startedAt, activeThread); err != nil {
		return err
	}
	if asJSON {
		return emitSessionJSON(cmd, sessionID, path, activeThread)
	}
	fmt.Fprintln(cmd.OutOrStdout(), path)
	return nil
}

// writeSessionFile renders a session file's frontmatter, writes it to path, and
// indexes it. Shared by `session create` and the Codex lazy-create in `session
// handoff`. created/retention derive from now; startedAt is caller-resolved so
// the create path's drift check and the file it writes agree on one value.
func writeSessionFile(v *core.Vault, path, sessionID, source, startedAt, activeThread string) (*core.Artifact, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	now := time.Now().UTC()
	data := templateData{
		Created:        now.Format("2006-01-02"),
		ShortID:        shortID(sessionID),
		Source:         source,
		SessionID:      sessionID,
		RetentionUntil: now.AddDate(0, 0, 30).Format("2006-01-02"),
		ActiveThread:   activeThread,
		StartedAt:      startedAt,
	}
	fm, err := renderFrontMatter(core.TypeSession, data)
	if err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}
	if err := schema.Validate(string(core.TypeSession), fm); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}
	a := &core.Artifact{Path: path, FrontMatter: fm}
	if err := a.Save(); err != nil {
		return nil, fmt.Errorf("saving artifact: %w", err)
	}
	if err := indexAfterSave(v, a); err != nil {
		return nil, fmt.Errorf("indexing %s: %w", sessionID, err)
	}
	return a, nil
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func sessionDrift(fm map[string]any, source, startedAt, activeThread string) string {
	if got, _ := fm["source"].(string); got != source {
		return "source"
	}
	if got, _ := fm["started_at"].(string); got != "" && got != startedAt {
		return "started_at"
	}
	related, _ := fm["related"].([]any)
	want := ""
	if activeThread != "" {
		want = "[[thread." + activeThread + "]]"
	}
	got := ""
	if len(related) > 0 {
		got, _ = related[0].(string)
	}
	if got != want {
		return "active-thread"
	}
	return ""
}

func emitSessionJSON(cmd *cobra.Command, id, path, activeThread string) error {
	related := []string{}
	if activeThread != "" {
		related = []string{"[[thread." + activeThread + "]]"}
	}
	out, err := json.Marshal(map[string]any{"id": id, "path": path, "related": related})
	if err != nil {
		return fmt.Errorf("marshalling json: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}
