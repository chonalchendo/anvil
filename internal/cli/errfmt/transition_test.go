package errfmt_test

import (
	"encoding/json"
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
		"code":       "illegal_transition",
		"type":       "issue",
		"id":         "demo.foo",
		"from":       "open",
		"to":         "resolved",
		"legal_next": []any{"in-progress", "abandoned"},
		"hint":       "anvil set issue demo.foo status resolved",
		"hint_note":  "force-edit: bypasses state machine, no audit trail",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("json mismatch (-want +got):\n%s", diff)
	}
}

func TestTransitionFlagRequiredErrorMessage(t *testing.T) {
	e := errfmt.NewTransitionFlagRequired("issue", "demo.foo", "open", "in-progress", "owner")
	if e.Error() == "" {
		t.Fatalf("Error() returned empty string")
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
