package errfmt_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

func TestStructured_JSON_PreservesFieldOrder(t *testing.T) {
	e := errfmt.NewStructured("illegal_transition").
		Set("type", "issue").
		Set("id", "demo.foo").
		Set("from", "open").
		Set("to", "resolved").
		Set("legal_next", []string{"in-progress", "abandoned"})

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"code":"illegal_transition","type":"issue","id":"demo.foo","from":"open","to":"resolved","legal_next":["in-progress","abandoned"]}`
	if string(b) != want {
		t.Errorf("json = %s\nwant = %s", string(b), want)
	}
}

func TestStructured_JSON_DecodesToExpectedShape(t *testing.T) {
	e := errfmt.NewStructured("transition_flag_required").
		Set("type", "issue").
		Set("id", "demo.foo").
		Set("flag", "owner").
		Set("required", true)

	b, _ := json.Marshal(e)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"code": "transition_flag_required",
		"type": "issue", "id": "demo.foo",
		"flag": "owner", "required": true,
	}
	if diff := cmp.Diff(want, parsed); diff != "" {
		t.Fatalf("decoded mismatch:\n%s", diff)
	}
}

func TestStructured_Error_RendersBracketTagged(t *testing.T) {
	e := errfmt.NewStructured("index_stale").Set("hint", "anvil reindex")
	want := "[index_stale]\n  hint: anvil reindex"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestStructured_Wrap_PreservesErrorsIs(t *testing.T) {
	sentinel := errors.New("sentinel")
	e := errfmt.NewStructured("foo").Wrap(sentinel)
	if !errors.Is(e, sentinel) {
		t.Errorf("errors.Is should match wrapped sentinel")
	}
}

func TestStructured_Set_OverwritesExistingKey(t *testing.T) {
	e := errfmt.NewStructured("foo").Set("a", "1").Set("a", "2")
	b, _ := json.Marshal(e)
	if string(b) != `{"code":"foo","a":"2"}` {
		t.Errorf("Set overwrite: %s", string(b))
	}
}
