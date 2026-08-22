package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// issueBodyWithIndirect builds a minimal valid issue body whose Indirect
// Verification block is the given line — the shape validateOne's TypeIssue
// case checks.
func issueBodyWithIndirect(indirectLine string) string {
	return "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\n" + indirectLine + "\n```\n\n## Links\n"
}

func TestValidate_SingleFile_HardcodedLakehouseSchema_Rejected(t *testing.T) {
	vault := setupVault(t)

	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "issue.foo.0001.bad-schema.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "bad schema", "created": "2026-08-06",
			"status": "open", "project": "foo", "goal": "fixed",
			"description": "test", "severity": "low", "tags": []any{"domain/vault"},
		},
		Body: issueBodyWithIndirect(`uv run mentat lakehouse query --explore "SELECT count(*) FROM lakehouse.modelled.met_fundamentals"`),
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", a.Path})
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected validate to refuse a hardcoded lakehouse schema, output: %s", out.String())
	}
	if !strings.Contains(out.String(), "lakehouse schema") {
		t.Errorf("output %q does not mention 'lakehouse schema'", out.String())
	}
}

func TestValidate_SingleFile_ParameterisedLakehouseSchema_Accepted(t *testing.T) {
	vault := setupVault(t)

	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "issue.foo.0002.good-schema.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "good schema", "created": "2026-08-06",
			"status": "open", "project": "foo", "goal": "fixed",
			"description": "test", "severity": "low", "tags": []any{"domain/vault"},
		},
		Body: issueBodyWithIndirect(`uv run mentat lakehouse query --explore "SELECT count(*) FROM lakehouse.${SCHEMA:-modelled}.met_fundamentals"`),
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", a.Path})
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parameterised schema must be accepted, got: %v\noutput: %s", err, out.String())
	}
}

func TestValidate_VerificationStdin_HardcodedLakehouseSchema(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", "--verification-stdin"})
	cmd.SetIn(strings.NewReader(`SELECT * FROM lakehouse.prod.orders`))
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected --verification-stdin to refuse a hardcoded lakehouse schema, output: %s", out.String())
	}
	if !strings.Contains(out.String(), "lakehouse schema") {
		t.Errorf("output %q does not mention 'lakehouse schema'", out.String())
	}
}
