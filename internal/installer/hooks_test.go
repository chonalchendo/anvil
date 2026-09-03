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
	if err := os.WriteFile(path, b, 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // path is test-controlled or application-managed; not user input
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
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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

const testEndCmd = `anvil session end --commit`

func TestMergeSessionEndHook_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := MergeSessionEndHook(path, testEndCmd)
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
	se, ok := hooks["SessionEnd"].([]any)
	if !ok || len(se) != 1 {
		t.Fatalf("SessionEnd = %v", hooks["SessionEnd"])
	}
}

func TestMergeSessionEndHook_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, err := MergeSessionEndHook(path, testEndCmd); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := MergeSessionEndHook(path, testEndCmd)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Error("changed = true on second merge, want false")
	}
}

func TestMergeSessionEndHook_ReplacesStaleVariant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionEndHook(path, testEndCmd); err != nil {
		t.Fatalf("seed old command: %v", err)
	}

	const newCmd = testEndCmd + " --push"
	changed, err := MergeSessionEndHook(path, newCmd)
	if err != nil {
		t.Fatalf("merge new command: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true when the managed command changes")
	}

	got := readJSON(t, path)
	se := got["hooks"].(map[string]any)["SessionEnd"].([]any)
	if len(se) != 1 {
		t.Fatalf("SessionEnd entries = %d, want 1 (stale variant not replaced): %v", len(se), se)
	}
	cmd := se[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"]
	if cmd != newCmd {
		t.Errorf("command = %q, want %q", cmd, newCmd)
	}
}

func TestMergeSessionEndHook_PreservesForeignHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionEndHook(path, "/usr/local/bin/my-own-hook.sh"); err != nil {
		t.Fatalf("seed foreign hook: %v", err)
	}

	if _, err := MergeSessionEndHook(path, testEndCmd); err != nil {
		t.Fatalf("merge anvil hook: %v", err)
	}

	got := readJSON(t, path)
	se := got["hooks"].(map[string]any)["SessionEnd"].([]any)
	if len(se) != 2 {
		t.Fatalf("SessionEnd entries = %d, want 2 (foreign hook clobbered): %v", len(se), se)
	}
	cmds := make(map[string]bool)
	for _, e := range se {
		cmds[e.(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)] = true
	}
	if !cmds["/usr/local/bin/my-own-hook.sh"] {
		t.Errorf("foreign hook not retained; surviving commands = %v", cmds)
	}
	if !cmds[testEndCmd] {
		t.Errorf("anvil hook not added; surviving commands = %v", cmds)
	}
}

func TestMergeSessionEndHook_PreservesSessionStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("seed SessionStart: %v", err)
	}

	if _, err := MergeSessionEndHook(path, testEndCmd); err != nil {
		t.Fatalf("merge SessionEnd: %v", err)
	}

	got := readJSON(t, path)
	hooks := got["hooks"].(map[string]any)
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart removed by MergeSessionEndHook")
	}
	if _, ok := hooks["SessionEnd"]; !ok {
		t.Error("SessionEnd hook not added")
	}
}

func TestRemoveSessionEndHook_RemovesMatching(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionEndHook(path, testEndCmd); err != nil {
		t.Fatalf("seed merge: %v", err)
	}

	changed, err := RemoveSessionEndHook(path, testEndCmd)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}

	got := readJSON(t, path)
	if hooks, ok := got["hooks"].(map[string]any); ok {
		if se, ok := hooks["SessionEnd"]; ok {
			if arr, ok := se.([]any); ok && len(arr) > 0 {
				t.Errorf("SessionEnd still present after remove: %v", arr)
			}
		}
	}
}

func TestRemoveSessionEndHook_MissingFileNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := RemoveSessionEndHook(path, testEndCmd)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("changed = true on missing file, want false")
	}
}

const (
	testMatcher    = "resume|compact"
	testMatcherCmd = "anvil install fire-session-resume"
)

func TestMergeSessionStartMatcherHook_CoexistsWithUnmatched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("seed unmatched: %v", err)
	}

	if _, err := MergeSessionStartMatcherHook(path, testMatcher, testMatcherCmd); err != nil {
		t.Fatalf("merge matcher hook: %v", err)
	}

	got := readJSON(t, path)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 2 {
		t.Fatalf("SessionStart len = %d, want 2 (unmatched + matcher-scoped)", len(ss))
	}
	var sawUnmatched, sawMatcher bool
	for _, e := range ss {
		m := e.(map[string]any)
		if _, hasMatcher := m["matcher"]; !hasMatcher {
			sawUnmatched = true
		} else if m["matcher"] == testMatcher {
			sawMatcher = true
		}
	}
	if !sawUnmatched || !sawMatcher {
		t.Errorf("expected both unmatched and matcher-scoped entries, got %v", ss)
	}
}

func TestMergeSessionStartMatcherHook_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionStartMatcherHook(path, testMatcher, testMatcherCmd); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := MergeSessionStartMatcherHook(path, testMatcher, testMatcherCmd)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Error("changed = true on second merge, want false")
	}
	got := readJSON(t, path)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Fatalf("SessionStart len = %d, want 1", len(ss))
	}
}

func TestRemoveSessionStartMatcherHook_KeepsUnmatched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeSessionStartHook(path, testCmd); err != nil {
		t.Fatalf("seed unmatched: %v", err)
	}
	if _, err := MergeSessionStartMatcherHook(path, testMatcher, testMatcherCmd); err != nil {
		t.Fatalf("seed matcher: %v", err)
	}

	changed, err := RemoveSessionStartMatcherHook(path, testMatcher, testMatcherCmd)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	got := readJSON(t, path)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Fatalf("SessionStart len = %d, want 1 (unmatched retained)", len(ss))
	}
	if _, hasMatcher := ss[0].(map[string]any)["matcher"]; hasMatcher {
		t.Errorf("remaining entry unexpectedly has a matcher: %v", ss[0])
	}
}

func TestMergePreCompactHook_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	const preCompactCmd = "anvil install fire-pre-compact"

	changed, err := MergePreCompactHook(path, preCompactCmd)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true on new file")
	}

	got := readJSON(t, path)
	hooks := got["hooks"].(map[string]any)
	pc, ok := hooks["PreCompact"].([]any)
	if !ok || len(pc) != 1 {
		t.Fatalf("PreCompact = %v", hooks["PreCompact"])
	}
}

func TestMergeSessionStartMatcherHook_DropsOrphanUnderDifferentMatcher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Simulate a prior install that scoped testMatcherCmd to a different
	// matcher (e.g. a since-edited matcher string).
	if _, err := MergeSessionStartMatcherHook(path, "old-matcher", testMatcherCmd); err != nil {
		t.Fatalf("seed stale matcher entry: %v", err)
	}

	changed, err := MergeSessionStartMatcherHook(path, testMatcher, testMatcherCmd)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true: stale-matcher orphan must be dropped")
	}

	got := readJSON(t, path)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Fatalf("SessionStart len = %d, want 1 (orphan dropped, new matcher entry kept)", len(ss))
	}
	m := ss[0].(map[string]any)
	if m["matcher"] != testMatcher {
		t.Errorf("surviving entry matcher = %v, want %q", m["matcher"], testMatcher)
	}
}

func TestMergeSessionStartMatcherHook_KeepsForeignEntryUnderDifferentMatcher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "old-matcher",
					"hooks":   []any{map[string]any{"type": "command", "command": "not-anvil-managed"}},
				},
			},
		},
	})

	if _, err := MergeSessionStartMatcherHook(path, testMatcher, testMatcherCmd); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	ss := got["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(ss) != 2 {
		t.Fatalf("SessionStart len = %d, want 2 (foreign entry preserved + new matcher entry)", len(ss))
	}
}

func TestMergeAutoCompactWindow_SetsWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := MergeAutoCompactWindow(path, 200000)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true on absent key")
	}

	got := readJSON(t, path)
	if got["autoCompactWindow"] != 200000.0 {
		t.Errorf("autoCompactWindow = %v, want 200000", got["autoCompactWindow"])
	}
}

func TestMergeAutoCompactWindow_NeverOverwritesExplicitValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{"autoCompactWindow": 50000})

	changed, err := MergeAutoCompactWindow(path, 200000)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if changed {
		t.Error("changed = true, want false: an explicit operator value must not be overwritten")
	}

	got := readJSON(t, path)
	if got["autoCompactWindow"] != 50000.0 {
		t.Errorf("autoCompactWindow = %v, want unchanged 50000", got["autoCompactWindow"])
	}
}

func TestRemoveAutoCompactWindow_RemovesInstallWrittenDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := MergeAutoCompactWindow(path, 200000); err != nil {
		t.Fatalf("seed: %v", err)
	}

	changed, err := RemoveAutoCompactWindow(path, 200000)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}

	got := readJSON(t, path)
	if _, exists := got["autoCompactWindow"]; exists {
		t.Errorf("autoCompactWindow still present after remove: %v", got["autoCompactWindow"])
	}
}

func TestRemoveAutoCompactWindow_PreservesOperatorChangedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, path, map[string]any{"autoCompactWindow": 50000})

	changed, err := RemoveAutoCompactWindow(path, 200000)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("changed = true, want false: an operator-changed value must not be removed")
	}

	got := readJSON(t, path)
	if got["autoCompactWindow"] != 50000.0 {
		t.Errorf("autoCompactWindow = %v, want unchanged 50000", got["autoCompactWindow"])
	}
}

func TestRemoveAutoCompactWindow_NoOpWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	changed, err := RemoveAutoCompactWindow(path, 200000)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("changed = true, want false: no key to remove")
	}
}
