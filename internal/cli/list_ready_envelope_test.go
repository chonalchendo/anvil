package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// list --ready --json (and the indexed --orphans path) must surface title,
// description, and severity. The JSON envelope is the agent's canonical
// read of the ready pool; missing fields force a second show/Read per item.
func TestListReadyJSON_IncludesTitleAndSeverity(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "fix login flake",
		"--description", "login intermittently fails",
		"--goal", "login flake is fixed",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	execCmd(t, "set", "issue", "demo.fix-login-flake", "severity", "high")

	out := execCmd(t, "list", "issue", "--ready", "--json")
	var env struct {
		Items []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Severity string `json:"severity"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if len(env.Items) != 1 {
		t.Fatalf("want 1 item, got %d: %s", len(env.Items), out)
	}
	got := env.Items[0]
	if got.Title != "fix login flake" {
		t.Errorf("title = %q, want %q", got.Title, "fix login flake")
	}
	if got.Severity != "high" {
		t.Errorf("severity = %q, want %q", got.Severity, "high")
	}
}

// --ready without --limit must return all ready issues, not just 10.
// The default cap of 10 is intentional for general list but wrong for the
// actionable ready queue — agents pick from it and invisible issues look blocked.
func TestListReadyJSON_DefaultLimitIsUnbounded(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	for i := range 12 {
		execCmd(t, "create", "issue",
			"--project", "demo",
			"--title", fmt.Sprintf("issue %d", i),
			"--description", "desc",
			"--goal", "goal",
			"--tags", "domain/dev-tools",
			"--allow-new-facet=domain",
		)
	}

	out := execCmd(t, "list", "issue", "--ready", "--json")
	var env struct {
		Items     []map[string]any `json:"items"`
		Total     int              `json:"total"`
		Returned  int              `json:"returned"`
		Truncated bool             `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if env.Total != 12 {
		t.Errorf("total = %d, want 12", env.Total)
	}
	if env.Returned != 12 {
		t.Errorf("returned = %d, want 12 (default --ready must not cap at 10)", env.Returned)
	}
	if env.Truncated {
		t.Errorf("truncated = true, want false when all items returned")
	}
}

// total must report the unbounded match count, not the post-limit slice
// length. truncated == (returned < total). Symptom 2 of the same issue.
func TestListReadyJSON_TotalIsUnboundedMatchCount(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	for _, title := range []string{"alpha one", "bravo two", "charlie three"} {
		execCmd(t, "create", "issue",
			"--project", "demo",
			"--title", title,
			"--description", "desc",
			"--goal", "goal",
			"--tags", "domain/dev-tools",
			"--allow-new-facet=domain",
		)
	}

	out := execCmd(t, "list", "issue", "--ready", "--json", "--limit", "2")
	var env struct {
		Items     []map[string]any `json:"items"`
		Total     int              `json:"total"`
		Returned  int              `json:"returned"`
		Truncated bool             `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if env.Total != 3 {
		t.Errorf("total = %d, want 3 (unbounded match count)", env.Total)
	}
	if env.Returned != 2 {
		t.Errorf("returned = %d, want 2 (post-limit slice)", env.Returned)
	}
	if !env.Truncated {
		t.Errorf("truncated = false, want true when returned < total")
	}
	if len(env.Items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(env.Items))
	}
}
