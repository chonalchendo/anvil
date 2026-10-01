package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// envSessionID is the per-terminal session identifier Claude Code exports into
// every subprocess it spawns. It is the deterministic, process-scoped handle
// that lets `anvil session` resolve "this terminal's session" without the
// mtime heuristic that lets concurrent sessions clobber each other's handoffs.
const envSessionID = "CLAUDE_CODE_SESSION_ID"

// envExplicitID and envExplicitSource let any harness bind a session without
// anvil guessing; source defaults to "other".
const (
	envExplicitID     = "ANVIL_SESSION_ID"
	envExplicitSource = "ANVIL_SESSION_SOURCE"
)

// resolveCurrentSession derives this terminal's session id and the path of its
// session file. The path is deterministic from the id; the file's existence is
// the caller's concern. source lets callers apply the right missing-file
// behaviour (Claude relies on the SessionStart hook; every other source has no
// hook and creates the file lazily).
func resolveCurrentSession() (id, path, source string, err error) {
	id, source, err = currentSessionBinding()
	if err != nil {
		return "", "", "", err
	}
	v, err := core.ResolveVault()
	if err != nil {
		return "", "", "", fmt.Errorf("resolving vault: %w", err)
	}
	return id, core.TypeSession.Path(v.Root, id), source, nil
}

// currentSessionBinding picks the id by precedence: harness-set signals first,
// guesses last. Claude and opencode set their own env, so they are trusted over
// $ANVIL_SESSION_ID, which a child session can inherit from a parent shell's
// export and would then merge distinct sessions into one file. Explicit is the
// fallback for harnesses anvil does not recognise. The Codex newest-rollout
// file is a guess (Codex exports no id), so it comes last and never shadows a
// harness that announced itself.
func currentSessionBinding() (id, source string, err error) {
	if id = os.Getenv(envSessionID); id != "" {
		return id, "claude-code", nil
	}
	if id = opencodeSessionID(); id != "" {
		return id, "opencode", nil
	}
	if id = os.Getenv(envExplicitID); id != "" {
		source = os.Getenv(envExplicitSource)
		if source == "" {
			return id, "other", nil
		}
		if !slices.Contains(validSessionSources, source) {
			return "", "", fmt.Errorf("%s=%q is not one of %s", envExplicitSource, source, strings.Join(validSessionSources, ", "))
		}
		return id, source, nil
	}
	if id, err = codexSessionID(); err == nil {
		return id, "codex", nil
	}
	return "", "", err
}

// opencodeSessionID keys on opencode's process id: its shell env carries only
// OPENCODE=1/OPENCODE_PID, and its on-disk session dirs are keyed by project,
// not by running process, so none can be mapped to this terminal reliably.
// The binding is therefore per opencode process, not per conversation: a
// restarted opencode gets a new id, and a recycled PID can land a later
// session on an earlier one's file (the integrity backstop only catches a
// mismatched stored id, not a reused one). Export ANVIL_SESSION_ID for a
// stable id.
func opencodeSessionID() string {
	if pid := os.Getenv("OPENCODE_PID"); pid != "" {
		return "opencode-" + pid
	}
	return ""
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
	var sawRollout bool
	//nolint:gosec // G703: root derives from $CODEX_HOME or the user's own home dir, not untrusted input
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // unreadable subtrees are skipped, not fatal
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		sawRollout = true // a rollout exists even if its name doesn't parse below
		if !codexRolloutID.MatchString(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is skipped, not fatal
		}
		if info.ModTime().After(newestMod) {
			newestMod, newestName = info.ModTime(), name
		}
		return nil
	})
	if newestName != "" {
		return codexRolloutID.FindStringSubmatch(newestName)[1], nil
	}
	// Split the two misses so a naming-format drift in Codex is diagnosable
	// rather than masquerading as "no session".
	if sawRollout {
		return "", fmt.Errorf("found Codex rollout files under %s but none matched the expected name rollout-<YYYY-MM-DDThh-mm-ss>-<id>.jsonl; report this so the binding can be fixed, or export %s=<stable id>", root, envExplicitID)
	}
	return "", fmt.Errorf("no active session: set %s, run under Codex (no rollout-*.jsonl under %s) or opencode, or for any other harness export %s=<stable id>", envSessionID, root, envExplicitID)
}
