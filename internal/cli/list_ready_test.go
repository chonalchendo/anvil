package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestListReadyFiltersUnblockedIssues(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)

	out := execCmd(t, "list", "issue", "--ready", "--json")
	var env struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	ids := make(map[string]bool)
	for _, it := range env.Items {
		ids[it.ID] = true
	}
	if !ids["demo.foo"] {
		t.Fatalf("expected demo.foo ready (no blocks edges yet): %v", env.Items)
	}
}

func TestListReadyRejectedForNonIssueType(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "milestone", "--ready"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected unsupported_for_type; out: %s err: %s", stdout.String(), stderr.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "unsupported_for_type") {
		t.Fatalf("expected unsupported_for_type code; got: %s", combined)
	}
}
