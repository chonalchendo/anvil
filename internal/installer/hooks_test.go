package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const testCmd = `anvil install fire-session-start`

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return got
}

func TestMergeSessionStartHook_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := MergeSessionStartHook(path, testCmd)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true on new file")
	}

	got := readJSON(t, path)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks key missing or wrong type: %v", got["hooks"])
	}
	ss, ok := hooks["SessionStart"].([]any)
	if !ok || len(ss) != 1 {
		t.Fatalf("SessionStart = %v", hooks["SessionStart"])
	}
}

func TestMergeSessionStartHook_PreservesUnrelatedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{
		"theme":      "dark",
		"otherStuff": map[string]any{"k": "v"},
	})

	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	if got["theme"] != "dark" {
		t.Errorf("theme = %v, want dark", got["theme"])
	}
	if diff := cmp.Diff(map[string]any{"k": "v"}, got["otherStuff"]); diff != "" {
		t.Errorf("otherStuff mismatch:\n%s", diff)
	}
}

func TestMergeSessionStartHook_PreservesOtherHookKinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo done"},
					},
				},
			},
		},
	})

	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	hooks := got["hooks"].(map[string]any)
	if _, ok := hooks["Stop"]; !ok {
		t.Error("Stop hook removed")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart hook not added")
	}
}

func TestMergeSessionStartHook_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := MergeSessionStartHook(path, testCmd)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Error("changed = true on second merge, want false")
	}
}

func TestMergeSessionStartHook_BadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MergeSessionStartHook(path, testCmd)
	if err == nil {
		t.Fatal("expected error on malformed settings.json")
	}
}

func TestRemoveSessionStartHook_RemovesMatching(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("seed merge: %v", err)
	}

	changed, err := RemoveSessionStartHook(path, testCmd)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}

	got := readJSON(t, path)
	if hooks, ok := got["hooks"].(map[string]any); ok {
		if ss, ok := hooks["SessionStart"]; ok {
			if arr, ok := ss.([]any); ok && len(arr) > 0 {
				t.Errorf("SessionStart still present after remove: %v", arr)
			}
		}
	}
}

func TestRemoveSessionStartHook_KeepsOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"hooks": []any{
					map[string]any{"type": "command", "command": "other-tool start"},
				}},
				map[string]any{"hooks": []any{
					map[string]any{"type": "command", "command": testCmd},
				}},
			},
		},
	})

	if _, err := RemoveSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got := readJSON(t, path)
	hooks := got["hooks"].(map[string]any)
	ss := hooks["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Fatalf("SessionStart len = %d, want 1", len(ss))
	}
	inner := ss[0].(map[string]any)["hooks"].([]any)
	if inner[0].(map[string]any)["command"] != "other-tool start" {
		t.Errorf("wrong entry retained: %v", inner)
	}
}

func TestRemoveSessionStartHook_MissingFileNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := RemoveSessionStartHook(path, testCmd)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("changed = true on missing file, want false")
	}
}
