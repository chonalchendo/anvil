package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestListReadyStrictExcludesBlockedAndBlockerTargets exercises the full
// readiness rule once the issue schema permits `depends_on`. With three
// issues — alpha (no edges), bravo (depends_on charlie), charlie (open) —
// only alpha is in the random-pickup pool: bravo is blocked by charlie, and
// charlie is excluded because someone (bravo) is already depending on it.
func TestListReadyStrictExcludesBlockedAndBlockerTargets(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "alpha", "--description", "alpha desc",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "bravo", "--description", "bravo desc",
		"--tags", "domain/dev-tools")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "charlie", "--description", "charlie desc",
		"--tags", "domain/dev-tools")
	execCmd(t, "set", "issue", "demo.bravo", "depends_on", "[[issue.demo.charlie]]")

	out := execCmd(t, "list", "issue", "--ready", "--json")
	var env struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	got := map[string]bool{}
	for _, it := range env.Items {
		got[it.ID] = true
	}
	if !got["demo.alpha"] {
		t.Errorf("expected demo.alpha ready; got %v", env.Items)
	}
	if got["demo.bravo"] {
		t.Errorf("demo.bravo has unresolved depends_on, should not be ready: %v", env.Items)
	}
	if got["demo.charlie"] {
		t.Errorf("demo.charlie is target of an open depends_on, should not be ready: %v", env.Items)
	}
}

// TestListReadyStrictRecoversWhenBlockerResolves confirms that resolving the
// blocker frees up the dependent issue.
func TestListReadyStrictRecoversWhenBlockerResolves(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "bravo", "--description", "bravo desc",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "charlie", "--description", "charlie desc",
		"--tags", "domain/dev-tools")
	execCmd(t, "set", "issue", "demo.bravo", "depends_on", "[[issue.demo.charlie]]")
	execCmd(t, "transition", "issue", "demo.charlie", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.charlie", "resolved")

	out := execCmd(t, "list", "issue", "--ready", "--json")
	if !strings.Contains(out, `"demo.bravo"`) {
		t.Errorf("demo.bravo should be ready once demo.charlie is resolved; got: %s", out)
	}
}
