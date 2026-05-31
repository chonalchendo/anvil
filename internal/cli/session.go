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
	cmd.AddCommand(
		newSessionCurrentCmd(),
		newSessionListCmd(),
		newSessionHandoffCmd(),
		newSessionShowCmd(),
		newSessionResumeCmd(),
	)
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
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", id, path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON object")
	return cmd
}

func newSessionListCmd() *cobra.Command {
	var asJSON bool
	var flagProject string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List session files with handoff metadata, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			items, err := collectSessions(v.Root, flagProject)
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
				fmt.Fprintln(w, "(no sessions)")
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
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", short, it.Modified, kind, firstNonEmpty(it.Objective, it.Title))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of session items (bare array, not the list envelope)")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter to sessions whose project matches this value")
	return cmd
}

func newSessionHandoffCmd() *cobra.Command {
	var flagBody, flagBodyFile, flagProject string
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
			if flagProject != "" {
				a.FrontMatter["project"] = flagProject
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
			fmt.Fprintf(cmd.OutOrStdout(), "Handoff written to %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBody, "body", "", "handoff body (literal, or - for stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read handoff body from a file")
	cmd.Flags().StringVar(&flagProject, "project", "", "stamp this project name onto the session file at handoff time")
	return cmd
}

// sessionShowOutput is the JSON envelope for `anvil session show <id>`.
type sessionShowOutput struct {
	SessionID  string  `json:"session_id"`
	Path       string  `json:"path"`
	Title      string  `json:"title,omitempty"`
	Modified   string  `json:"modified"`
	HasHandoff bool    `json:"has_handoff"`
	Body       *string `json:"body,omitempty"`
}

func newSessionShowCmd() *cobra.Command {
	var flagBody, flagJSON bool
	cmd := &cobra.Command{
		Use:   "show <session_id>",
		Short: "Print metadata (and optionally body) for a session by its session_id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wantID := args[0]
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			items, err := collectSessions(v.Root, "")
			if err != nil {
				return err
			}
			// Full match first; fall back to prefix match for convenience but
			// require exactly one prefix match to avoid silent mis-resolution.
			var matched *sessionItem
			var prefixMatches []string
			for i := range items {
				if items[i].SessionID == wantID {
					matched = &items[i]
					break
				}
				if strings.HasPrefix(items[i].SessionID, wantID) {
					prefixMatches = append(prefixMatches, items[i].SessionID)
				}
			}
			if matched == nil && len(prefixMatches) == 1 {
				for i := range items {
					if items[i].SessionID == prefixMatches[0] {
						matched = &items[i]
						break
					}
				}
			}
			if matched == nil {
				// Actionable error: names the field, states accepted form, suggests running list.
				if len(prefixMatches) > 1 {
					return fmt.Errorf("session_id %q is ambiguous; matching sessions: %s — use the full session_id from `anvil session list`", wantID, strings.Join(prefixMatches, ", "))
				}
				return fmt.Errorf("session_id %q not found; session_id values are full UUIDs — run `anvil session list` to see valid ids", wantID)
			}
			out := sessionShowOutput{
				SessionID:  matched.SessionID,
				Path:       matched.Path,
				Title:      matched.Title,
				Modified:   matched.Modified,
				HasHandoff: matched.HasHandoff,
			}
			if flagBody {
				a, err := core.LoadArtifact(matched.Path)
				if err != nil {
					return fmt.Errorf("loading session file: %w", err)
				}
				body := a.Body
				out.Body = &body
			}
			if flagJSON {
				return writeJSON(cmd, out)
			}
			w := cmd.OutOrStdout()
			if flagBody && out.Body != nil {
				fmt.Fprint(w, *out.Body)
				return nil
			}
			kind := "stub"
			if out.HasHandoff {
				kind = "handoff"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.SessionID, out.Modified, kind, out.Title)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagBody, "body", false, "include the full session body in the output")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON object")
	return cmd
}

// resumeOutput is the JSON envelope for `anvil session resume`.
type resumeOutput struct {
	SessionID  string        `json:"session_id"`
	Path       string        `json:"path"`
	Objective  string        `json:"objective,omitempty"`
	Project    string        `json:"project,omitempty"`
	Body       string        `json:"body"`
	Walked     int           `json:"walked"`
	Candidates []sessionItem `json:"candidates,omitempty"`
}

const resumeAmbiguityWindowSecs = 600

func newSessionResumeCmd() *cobra.Command {
	var flagJSON bool
	var flagProject string
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Return the most-recent handoff, disambiguating when ≥2 landed within the 10-min ambiguity window",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			items, err := collectSessions(v.Root, flagProject)
			if err != nil {
				return err
			}

			// walked = number of leading stubs (no handoff) before the first handoff.
			walked := 0
			firstHandoffIdx := -1
			for i, it := range items {
				if it.HasHandoff {
					firstHandoffIdx = i
					walked = i
					break
				}
			}
			if firstHandoffIdx == -1 {
				// When scoped to a project with --json, return an empty result
				// so callers can test for absence without error-handling the exit code.
				if flagJSON && flagProject != "" {
					return writeJSON(cmd, resumeOutput{Walked: walked, Candidates: []sessionItem{}})
				}
				if flagProject != "" {
					return fmt.Errorf("no prior handoff found for project %q", flagProject)
				}
				return fmt.Errorf("no prior handoff found — no session file under the vault has a non-empty body")
			}

			newestHandoff := items[firstHandoffIdx]

			// Collect all handoffs in the ambiguity window relative to the newest.
			newestTime, err := time.Parse(time.RFC3339, newestHandoff.Modified)
			if err != nil {
				return fmt.Errorf("parsing modified time %q: %w", newestHandoff.Modified, err)
			}
			candidates := []sessionItem{}
			for _, it := range items {
				if !it.HasHandoff {
					continue
				}
				t, err := time.Parse(time.RFC3339, it.Modified)
				if err != nil {
					continue
				}
				if newestTime.Sub(t).Seconds() <= resumeAmbiguityWindowSecs {
					candidates = append(candidates, it)
				}
			}

			if len(candidates) > 1 {
				// Return the candidate list for the caller to disambiguate.
				out := resumeOutput{
					Walked:     walked,
					Candidates: candidates,
					Body:       "",
				}
				if flagJSON {
					return writeJSON(cmd, out)
				}
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, "Multiple recent handoffs in the ambiguity window:")
				for i, c := range candidates {
					fmt.Fprintf(w, "  %d) %s  %s  %s\n", i+1, c.SessionID[:8], c.Modified, firstNonEmpty(c.Objective, c.Title))
				}
				fmt.Fprintln(w, "Use `anvil session show <session_id> --body` to load a specific handoff.")
				return nil
			}

			chosen := candidates[0]
			a, err := core.LoadArtifact(chosen.Path)
			if err != nil {
				return fmt.Errorf("loading session file: %w", err)
			}
			out := resumeOutput{
				SessionID: chosen.SessionID,
				Path:      chosen.Path,
				Objective: chosen.Objective,
				Project:   chosen.Project,
				Body:      a.Body,
				Walked:    walked,
			}
			if flagJSON {
				return writeJSON(cmd, out)
			}
			fmt.Fprint(cmd.OutOrStdout(), a.Body)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON object")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter candidates to sessions whose project matches this value")
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
	Project    string `json:"project,omitempty"`
	Modified   string `json:"modified"`
	HasHandoff bool   `json:"has_handoff"`
}

// collectSessions returns session items from the vault, newest-first.
// When filterProject is non-empty only sessions whose project frontmatter
// matches are returned.
func collectSessions(vaultRoot, filterProject string) ([]sessionItem, error) {
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
		project, _ := a.FrontMatter["project"].(string)
		if filterProject != "" && project != filterProject {
			continue
		}
		items = append(items, sessionItem{
			SessionID:  id,
			Path:       path,
			Title:      title,
			Objective:  parseObjective(a.Body),
			Project:    project,
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
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}
