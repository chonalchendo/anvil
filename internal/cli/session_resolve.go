package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// envSessionID is the per-terminal session identifier Claude Code exports into
// every subprocess it spawns. It is the deterministic, process-scoped handle
// that lets `anvil session` resolve "this terminal's session" without the
// mtime heuristic that lets concurrent sessions clobber each other's handoffs.
const envSessionID = "CLAUDE_CODE_SESSION_ID"

// resolveCurrentSession derives this terminal's session id and the path of its
// session file. It prefers Claude Code's envSessionID; failing that it binds to
// the active Codex session, which exports no session-id env var but persists a
// per-session rollout file we can read. The path is deterministic from the id;
// the file's existence is the caller's concern. source distinguishes the two so
// callers can apply the right missing-file behaviour (Claude relies on the
// SessionStart hook; Codex has none and creates the file lazily).
func resolveCurrentSession() (id, path, source string, err error) {
	id, source = os.Getenv(envSessionID), "claude-code"
	if id == "" {
		id, err = codexSessionID()
		if err != nil {
			return "", "", "", err
		}
		source = "codex"
	}
	v, err := core.ResolveVault()
	if err != nil {
		return "", "", "", fmt.Errorf("resolving vault: %w", err)
	}
	return id, core.TypeSession.Path(v.Root, "", id), source, nil
}

// codexRolloutID extracts the session id trailing the timestamp in a Codex
// rollout filename (rollout-<RFC3339-ish>-<id>.jsonl). Matching only the fixed
// timestamp prefix keeps this agnostic to the id's internal shape.
var codexRolloutID = regexp.MustCompile(`^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-(.+)\.jsonl$`)

// codexSessionID returns the active Codex session's id, read from its newest
// rollout transcript under $CODEX_HOME/sessions (default ~/.codex/sessions).
// Codex exports no session-id env var (openai/codex#8923); the newest rollout
// file is the live session, mirroring the single-active-session assumption the
// Claude path already makes.
func codexSessionID() (string, error) {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		home = filepath.Join(h, ".codex")
	}
	root := filepath.Join(home, "sessions")
	var newestName string
	var newestMod time.Time
	//nolint:gosec // G703: root derives from $CODEX_HOME or the user's own home dir, not untrusted input
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // unreadable subtrees are skipped, not fatal
		}
		if !codexRolloutID.MatchString(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is skipped, not fatal
		}
		if info.ModTime().After(newestMod) {
			newestMod, newestName = info.ModTime(), d.Name()
		}
		return nil
	})
	if newestName == "" {
		return "", fmt.Errorf("no active session: set %s, or run under Codex (no rollout file under %s)", envSessionID, root)
	}
	return codexRolloutID.FindStringSubmatch(newestName)[1], nil
}
