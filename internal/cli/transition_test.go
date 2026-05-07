package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createDemoIssue(t *testing.T, vault string) {
	t.Helper()
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
}

func TestTransitionHappyPathWritesFrontmatter(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)

	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if got["status"] != "transitioned" || got["from"] != "open" || got["to"] != "in-progress" || got["owner"] != "claude" {
		t.Fatalf("envelope: %v", got)
	}

	row, err := openIndex(t, vault).GetArtifact("demo.foo")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "in-progress" {
		t.Fatalf("index status: %q", row.Status)
	}
}

func TestTransitionIdempotentWhenAlreadyInState(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)

	out := execCmd(t, "transition", "issue", "demo.foo", "open", "--json")
	if !strings.Contains(out, `"already_in_state"`) {
		t.Fatalf("expected already_in_state, got %s", out)
	}
}

func TestTransitionIllegalReturnsErr(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected illegal_transition error; output: %s", out.String())
	}
	if !strings.Contains(out.String(), "illegal_transition") {
		t.Fatalf("expected error code in output: %s", out.String())
	}
}

func TestTransitionMissingRequiredFlag(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected transition_flag_required; output: %s", out.String())
	}
	if !strings.Contains(out.String(), "owner") {
		t.Fatalf("expected `owner` mentioned: %s", out.String())
	}
}

func TestTransitionReverseRequiresReason(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "open"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected reason required; output: %s", out.String())
	}
}

func TestTransitionReverseAppendsAuditLine(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")
	execCmd(t, "transition", "issue", "demo.foo", "open", "--reason", "regression found")

	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "regression found") {
		t.Fatalf("audit line missing in body:\n%s", body)
	}
}
