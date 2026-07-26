package errfmt_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

func TestIllegalTransitionJSON(t *testing.T) {
	e := errfmt.NewIllegalTransition("issue", "demo.foo", "open", "resolved",
		[]string{"in-progress", "abandoned"})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"code":          "illegal_transition",
		"type":          "issue",
		"id":            "demo.foo",
		"from":          "open",
		"to":            "resolved",
		"legal_next":    []any{"in-progress", "abandoned"},
		"hint":          "anvil set issue demo.foo status resolved",
		"hint_note":     "force-edit: bypasses state machine, no audit trail",
		"sequence_hint": "anvil transition issue demo.foo in-progress --owner <name> && anvil transition issue demo.foo resolved",
		"sequence_note": "claim records ownership and in-progress duration before resolution; force-edit skips this audit trail",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("json mismatch (-want +got):\n%s", diff)
	}
}

func TestIllegalTransitionJSON_NoSequenceHintForOtherEdges(t *testing.T) {
	e := errfmt.NewIllegalTransition("plan", "demo.foo", "draft", "done", []string{"locked", "abandoned"})
	b, _ := json.Marshal(e)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["sequence_hint"]; ok {
		t.Errorf("sequence_hint should only appear on issue: open→resolved, got %v", got)
	}
	if _, ok := got["sequence_note"]; ok {
		t.Errorf("sequence_note should only appear on issue: open→resolved, got %v", got)
	}
}

func TestTransitionFlagRequiredErrorMessage(t *testing.T) {
	e := errfmt.NewTransitionFlagRequired("issue", "demo.foo", "open", "in-progress", "owner")
	if e.Error() == "" {
		t.Fatalf("Error() returned empty string")
	}
}

func TestNewIndexStaleCarriesPathInDetail(t *testing.T) {
	e := errfmt.NewIndexStale("vault index stale: 70-issues/demo.0001.x.md no longer exists on disk (last reindex 2026-07-26T00:00:00Z)")
	if !strings.Contains(e.Error(), "demo.0001.x.md") {
		t.Fatalf("Error() must surface the offending path, got %q", e.Error())
	}
	b, _ := json.Marshal(e)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["code"] != "index_stale" {
		t.Errorf("index_stale JSON: %v", parsed)
	}
	detail, ok := parsed["detail"].(string)
	if !ok || !strings.Contains(detail, "demo.0001.x.md") {
		t.Errorf("expected detail field naming the path, got %v", parsed)
	}
}

func TestNewIndexStale_NoDetailOmitsField(t *testing.T) {
	e := errfmt.NewIndexStale("")
	b, _ := json.Marshal(e)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["detail"]; ok {
		t.Errorf("detail should be omitted when empty, got %v", parsed)
	}
}

func TestInvalidSlug_JSONShape(t *testing.T) {
	e := errfmt.NewInvalidSlug("Bad Slug", nil)
	b, _ := json.Marshal(e)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["code"] != "invalid_slug" || parsed["slug"] != "Bad Slug" {
		t.Errorf("invalid_slug JSON: %v", parsed)
	}
	if _, ok := parsed["pattern"].(string); !ok {
		t.Errorf("missing pattern field: %v", parsed)
	}
}
