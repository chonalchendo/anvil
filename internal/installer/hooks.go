package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// anvilHookPrefix identifies hook entries anvil manages. Anvil owns exactly one
// hook per managed event, so install upserts: any entry invoking `anvil ` is a
// prior managed command and is replaced — not duplicated — when the command
// string changes (e.g. a new flag added to the SessionEnd hook).
const anvilHookPrefix = "anvil "

// MergeSessionStartHook registers command under the Claude Code SessionStart
// hook event in settingsPath.
func MergeSessionStartHook(settingsPath, command string) (bool, error) {
	return mergeHook(settingsPath, "SessionStart", command)
}

// RemoveSessionStartHook strips command from the SessionStart hook event in
// settingsPath.
func RemoveSessionStartHook(settingsPath, command string) (bool, error) {
	return removeHook(settingsPath, "SessionStart", command)
}

// MergeSessionEndHook registers command under the Claude Code SessionEnd hook
// event in settingsPath.
func MergeSessionEndHook(settingsPath, command string) (bool, error) {
	return mergeHook(settingsPath, "SessionEnd", command)
}

// RemoveSessionEndHook strips command from the SessionEnd hook event in
// settingsPath.
func RemoveSessionEndHook(settingsPath, command string) (bool, error) {
	return removeHook(settingsPath, "SessionEnd", command)
}

// mergeHook ensures settingsPath contains a Claude Code hook for the given
// event that runs command. The file is created if missing. Unrelated keys and
// non-anvil hook entries are preserved; any stale anvil-managed entry (a prior
// command string) is replaced so a changed command upserts instead of
// accumulating a duplicate that double-fires. Returns changed=false only when
// command is already the sole anvil entry and nothing stale needed dropping.
func mergeHook(settingsPath, event, command string) (bool, error) {
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return false, err
	}

	hooks := getOrCreateMap(settings, "hooks")
	entries := getOrCreateSlice(hooks, event)

	kept := make([]any, 0, len(entries))
	hasCurrent := false
	for _, e := range entries {
		switch {
		case entryMatchesCommand(e, command):
			hasCurrent = true
			kept = append(kept, e)
		case entryIsManaged(e):
			continue // drop a stale anvil-managed variant
		default:
			kept = append(kept, e)
		}
	}
	if hasCurrent && len(kept) == len(entries) {
		return false, nil
	}
	if !hasCurrent {
		kept = append(kept, map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": command},
			},
		})
	}
	hooks[event] = kept
	settings["hooks"] = hooks

	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}

// removeHook strips any hook entry under event whose inner command matches
// command. Missing file or missing hook is not an error.
func removeHook(settingsPath, event, command string) (bool, error) {
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return false, err
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}
	entries, ok := hooks[event].([]any)
	if !ok {
		return false, nil
	}

	kept := make([]any, 0, len(entries))
	changed := false
	for _, e := range entries {
		if entryMatchesCommand(e, command) {
			changed = true
			continue
		}
		kept = append(kept, e)
	}
	if !changed {
		return false, nil
	}
	hooks[event] = kept

	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}

func loadSettings(path string) (map[string]any, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func writeSettings(path string, m map[string]any) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func getOrCreateMap(parent map[string]any, key string) map[string]any {
	if v, ok := parent[key].(map[string]any); ok {
		return v
	}
	m := map[string]any{}
	parent[key] = m
	return m
}

func getOrCreateSlice(parent map[string]any, key string) []any {
	if v, ok := parent[key].([]any); ok {
		return v
	}
	return []any{}
}

func entryMatchesCommand(entry any, command string) bool {
	return entryCommandMatches(entry, func(c string) bool { return c == command })
}

// entryIsManaged reports whether entry invokes an anvil-managed command, by the
// anvilHookPrefix rule — the identity install upserts on.
func entryIsManaged(entry any) bool {
	return entryCommandMatches(entry, func(c string) bool { return strings.HasPrefix(c, anvilHookPrefix) })
}

// entryCommandMatches reports whether any command inside a Claude Code hook
// entry satisfies pred.
func entryCommandMatches(entry any, pred func(string) bool) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	inner, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range inner {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if c, ok := hm["command"].(string); ok && pred(c) {
			return true
		}
	}
	return false
}
