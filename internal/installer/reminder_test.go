package installer

import (
	"path/filepath"
	"testing"
)

func TestMergeReminderSuppression_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := MergeReminderSuppression(path)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !changed {
		t.Error("changed = false on new file, want true")
	}

	got := readJSON(t, path)
	env, ok := got["env"].(map[string]any)
	if !ok {
		t.Fatalf("env missing or wrong type: %v", got["env"])
	}
	if env["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"] != "false" {
		t.Errorf("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION = %v, want false", env["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"])
	}
}

func TestMergeReminderSuppression_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, err := MergeReminderSuppression(path); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := MergeReminderSuppression(path)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Error("changed = true on second merge, want false")
	}
}

func TestMergeReminderSuppression_PreservesUnrelatedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{
		"theme": "dark",
		"env":   map[string]any{"EXISTING_VAR": "1"},
	})

	if _, err := MergeReminderSuppression(path); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	if got["theme"] != "dark" {
		t.Errorf("theme = %v, want dark", got["theme"])
	}
	env := got["env"].(map[string]any)
	if env["EXISTING_VAR"] != "1" {
		t.Errorf("EXISTING_VAR = %v, want 1", env["EXISTING_VAR"])
	}
	if env["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"] != "false" {
		t.Errorf("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION = %v, want false", env["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"])
	}
}

func TestRemoveReminderSuppression_RemovesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, err := MergeReminderSuppression(path); err != nil {
		t.Fatalf("seed: %v", err)
	}
	changed, err := RemoveReminderSuppression(path)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true after remove")
	}

	got := readJSON(t, path)
	if env, ok := got["env"].(map[string]any); ok {
		if _, present := env["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"]; present {
			t.Error("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION still present after remove")
		}
	}
}

func TestRemoveReminderSuppression_MissingFileNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := RemoveReminderSuppression(path)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("changed = true on missing file, want false")
	}
}

func TestRemoveReminderSuppression_KeyAbsentNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{"theme": "dark"})

	changed, err := RemoveReminderSuppression(path)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("changed = true when key absent, want false")
	}
}
