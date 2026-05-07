package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPromoteProjectFlagOverridesResolver confirms `--project` on `promote`
// pins the promoted issue to the supplied slug, ignoring the resolver and
// the inbox's `suggested_project`. Without it, agents working in arbitrary
// CWDs (e.g. the anvil repo itself) get the wrong project on the issue.
func TestPromoteProjectFlagOverridesResolver(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	out := execCmd(t, "create", "inbox", "--title", "fix-the-thing", "--description", "drive-by", "--json")
	var created struct{ ID string }
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &created); err != nil {
		t.Fatalf("create json: %v\nout: %s", err, out)
	}

	promoteOut := execCmd(t, "promote", created.ID,
		"--as", "issue",
		"--project", "demo",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json")
	var p struct {
		TargetID *string `json:"target_id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(promoteOut)), &p); err != nil {
		t.Fatalf("promote json: %v\nout: %s", err, promoteOut)
	}
	if p.TargetID == nil {
		t.Fatalf("no target_id in promote output: %s", promoteOut)
	}
	if !strings.HasPrefix(*p.TargetID, "demo.") {
		t.Errorf("--project demo should produce target_id `demo.<slug>`, got %q", *p.TargetID)
	}
}
