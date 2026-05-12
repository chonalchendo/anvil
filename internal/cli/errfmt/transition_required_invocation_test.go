package errfmt

import (
	"encoding/json"
	"strings"
	"testing"
)

// The transition_flag_required error must include a copy-pasteable corrected
// invocation per agent-cli-principles rule 4. This is the canonical example:
// missing --owner on the open → in-progress edge.
func TestTransitionFlagRequired_IncludesCorrectedInvocation(t *testing.T) {
	e := NewTransitionFlagRequired("issue", "demo.foo", "open", "in-progress", "owner")

	if !strings.Contains(e.Error(), "corrected:") {
		t.Errorf("text error missing 'corrected:' line:\n%s", e.Error())
	}
	wantCmd := "anvil transition issue demo.foo in-progress --owner <name>"
	if !strings.Contains(e.Error(), wantCmd) {
		t.Errorf("text error missing corrected invocation %q:\n%s", wantCmd, e.Error())
	}

	b, _ := json.Marshal(e)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["corrected"] != wantCmd {
		t.Errorf("json `corrected` = %v, want %q", got["corrected"], wantCmd)
	}
	if got["required"] != true {
		t.Errorf("json `required` = %v, want true", got["required"])
	}
}
