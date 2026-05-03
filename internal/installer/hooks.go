package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// MergeSessionStartHook ensures settingsPath contains a Claude Code
// SessionStart hook that runs command. The file is created if missing.
// All unrelated keys and existing hook entries are preserved.
// Returns changed=false if the exact command was already present.
func MergeSessionStartHook(settingsPath, command string) (bool, error) {
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return false, err
	}

	hooks := getOrCreateMap(settings, "hooks")
	ss := getOrCreateSlice(hooks, "SessionStart")

	if hookCommandPresent(ss, command) {
		return false, nil
	}

	entry := map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	}
	hooks["SessionStart"] = append(ss, entry)
	settings["hooks"] = hooks

	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveSessionStartHook strips any SessionStart hook entry whose inner
// command matches command. Missing file or missing hook is not an error.
func RemoveSessionStartHook(settingsPath, command string) (bool, error) {
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return false, err
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}
	ss, ok := hooks["SessionStart"].([]any)
	if !ok {
		return false, nil
	}

	kept := make([]any, 0, len(ss))
	changed := false
	for _, e := range ss {
		if entryMatchesCommand(e, command) {
			changed = true
			continue
		}
		kept = append(kept, e)
	}
	if !changed {
		return false, nil
	}
	hooks["SessionStart"] = kept

	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}

func loadSettings(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
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
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
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

func hookCommandPresent(entries []any, command string) bool {
	for _, e := range entries {
		if entryMatchesCommand(e, command) {
			return true
		}
	}
	return false
}

func entryMatchesCommand(entry any, command string) bool {
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
		if hm["command"] == command {
			return true
		}
	}
	return false
}
