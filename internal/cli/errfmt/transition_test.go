package errfmt

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIllegalTransitionJSON(t *testing.T) {
	e := IllegalTransition{
		Code: "illegal_transition",
		Type: "issue", ID: "demo.foo",
		From: "open", To: "resolved",
		LegalNext: []string{"in-progress", "abandoned"},
	}
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
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("json mismatch (-want +got):\n%s", diff)
	}
}

func TestTransitionFlagRequiredErrorMessage(t *testing.T) {
	e := TransitionFlagRequired{Type: "issue", ID: "demo.foo", From: "open", To: "in-progress", Flag: "owner"}
	if e.Error() == "" {
		t.Fatalf("Error() returned empty string")
	}
}
