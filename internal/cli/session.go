package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// envSessionID is the per-terminal session identifier Claude Code exports into
// every subprocess it spawns. It is the deterministic, process-scoped handle
// that lets `anvil session` resolve "this terminal's session" without the
// mtime heuristic that lets concurrent sessions clobber each other's handoffs.
const envSessionID = "CLAUDE_CODE_SESSION_ID"

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Resolve the current Claude Code session and write its handoff",
	}
	cmd.AddCommand(newSessionCurrentCmd(), newSessionListCmd(), newSessionHandoffCmd())
	return cmd
}

func newSessionCurrentCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the invoking terminal's session id and file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, path, err := resolveCurrentSession()
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, map[string]string{"session_id": id, "path": path})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", id, path) //nolint:errcheck // cobra writer methods ignore write errors by design
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON object")
	return cmd
}

func newSessionListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List session files with handoff metadata, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			items, err := collectSessions(v.Root)
			if err != nil {
				return err
			}
			if asJSON {
				// A plain array, not the `anvil list` {items,total,...} envelope:
				// this is a fixed-cardinality internal feed (one row per session
				// file, GC'd separately) that resuming-session consumes with
				// `jq 'map(...)'` / `.[0]`. No limit or truncation hint applies.
				return writeJSON(cmd, items)
			}
			w := cmd.OutOrStdout()
			if len(items) == 0 {
				fmt.Fprintln(w, "(no sessions)") //nolint:errcheck // cobra writer methods ignore write errors by design
				return nil
			}
			for _, it := range items {
				kind := "stub"
				if it.HasHandoff {
					kind = "handoff"
				}
				short := it.SessionID
				if len(short) > 8 {
					short = short[:8]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", short, it.Modified, kind, firstNonEmpty(it.Objective, it.Title)) //nolint:errcheck // cobra writer methods ignore write errors by design
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array")
	return cmd
}

func newSessionHandoffCmd() *cobra.Command {
	var flagBody, flagBodyFile string
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Write the handoff body into the current session file (refuses a file owned by another session)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := readBody(cmd, flagBody, flagBodyFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(body) == "" {
				return errors.New("handoff body is empty; supply --body, --body-file, or piped stdin")
			}
			id, path, err := resolveCurrentSession()
			if err != nil {
				return err
			}
			a, err := core.LoadArtifact(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("session file %s not found; is the SessionStart hook installed?", path)
				}
				return fmt.Errorf("loading %s: %w", path, err)
			}
			// Deriving path from envSessionID is what prevents the cross-session
			// clobber: each terminal writes only to its own <id>.md, so two live
			// sessions never target one file. This check is the integrity backstop
			// — it refuses a file at the derived path whose stored id disagrees
			// (a renamed, hand-edited, or otherwise foreign file), rather than
			// silently overwriting it.
			if stored, _ := a.FrontMatter["session_id"].(string); stored != id {
				return fmt.Errorf("refusing handoff: %s stores session %q, not current %q — its session_id disagrees with the resolved path", path, stored, id)
			}
			a.Body = body
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving handoff: %w", err)
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("reindexing %s: %w", id, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Handoff written to %s\n", path) //nolint:errcheck // cobra writer methods ignore write errors by design
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBody, "body", "", "handoff body (literal, or - for stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read handoff body from a file")
	return cmd
}

// resolveCurrentSession derives this terminal's session id from envSessionID
// and composes the path of its session file. The path is deterministic from
// the env var alone; the file's existence is the caller's concern.
func resolveCurrentSession() (id, path string, err error) {
	id = os.Getenv(envSessionID)
	if id == "" {
		return "", "", fmt.Errorf("not running inside a Claude Code session (%s unset)", envSessionID)
	}
	v, err := core.ResolveVault()
	if err != nil {
		return "", "", fmt.Errorf("resolving vault: %w", err)
	}
	return id, core.TypeSession.Path(v.Root, "", id), nil
}

// sessionItem is one row of `anvil session list`, carrying the metadata
// resuming-session needs to disambiguate near-simultaneous handoffs.
type sessionItem struct {
	SessionID  string `json:"session_id"`
	Path       string `json:"path"`
	Title      string `json:"title,omitempty"`
	Objective  string `json:"objective,omitempty"`
	Modified   string `json:"modified"`
	HasHandoff bool   `json:"has_handoff"`
}

func collectSessions(vaultRoot string) ([]sessionItem, error) {
	dir := filepath.Join(vaultRoot, core.TypeSession.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []sessionItem{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	items := []sessionItem{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		a, err := core.LoadArtifact(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", filepath.Base(path), err)
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		if sid, _ := a.FrontMatter["session_id"].(string); sid != "" {
			id = sid
		}
		title, _ := a.FrontMatter["title"].(string)
		items = append(items, sessionItem{
			SessionID:  id,
			Path:       path,
			Title:      title,
			Objective:  parseObjective(a.Body),
			Modified:   info.ModTime().UTC().Format(time.RFC3339),
			HasHandoff: strings.TrimSpace(a.Body) != "",
		})
	}
	// RFC3339 timestamps sort lexically in chronological order; reverse for
	// newest-first, matching resuming-session's recency walk.
	sort.Slice(items, func(i, j int) bool { return items[i].Modified > items[j].Modified })
	return items, nil
}

// parseObjective extracts the text after the handoff body's `**Objective.**`
// marker — the one-sentence campaign goal resuming-session surfaces first.
// Returns "" for stubs and pre-Objective handoffs.
func parseObjective(body string) string {
	const marker = "**Objective.**"
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, marker); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func writeJSON(cmd *cobra.Command, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling json: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck // cobra writer methods ignore write errors by design
	return nil
}
