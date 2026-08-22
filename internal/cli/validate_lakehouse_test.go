package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
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

// TestCreateIssue_HardcodedLakehouseSchema_Rejected drives the create-gate
// site (staticBodyFailures, not validateOne) — the third wired call site,
// previously untested. It must refuse and roll back, the way
// TestCreate_Issue_BodyFile_RejectsBadWikilink_RollsBack does for the
// sibling wikilink check.
func TestCreateIssue_HardcodedLakehouseSchema_Rejected(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	body := issueBodyWithIndirect(`uv run mentat lakehouse query --explore "SELECT count(*) FROM lakehouse.modelled.met_fundamentals"`)
	bodyPath := filepath.Join(t.TempDir(), "issue-body.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "issue",
		"--title", "bad-schema",
		"--description", "test",
		"--goal", "goal",
		"--body-file", bodyPath,
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("err = %v, want ErrSchemaInvalid", err)
	}
	if !strings.Contains(stderr.String(), "lakehouse schema") {
		t.Errorf("stderr should mention 'lakehouse schema', got: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "foo.bad-schema.md")); !os.IsNotExist(err) {
		t.Errorf("file should be rolled back; stat err = %v", err)
	}
}

// TestValidate_Sweep_HardcodedLakehouseSchema_WarnsNotFails proves the
// warning tier: the multi-file vault-wide sweep must still surface a
// hardcoded schema (visible in --json output) but not fail the run, while
// single-file validate on the same file (above) refuses outright.
func TestValidate_Sweep_HardcodedLakehouseSchema_WarnsNotFails(t *testing.T) {
	vault := setupVault(t)

	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "issue.foo.0003.bad-schema-sweep.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "bad schema sweep", "created": "2026-08-06",
			"status": "open", "project": "foo", "goal": "fixed",
			"description": "test", "severity": "low", "tags": []any{"domain/vault"},
		},
		Body: issueBodyWithIndirect(`uv run mentat lakehouse query --explore "SELECT count(*) FROM lakehouse.modelled.met_fundamentals"`),
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault, "--json"})
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sweep must not fail on a lakehouse-schema warning, got: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), errfmt.SeverityWarning) {
		t.Errorf("sweep --json output should carry severity %q, got: %s", errfmt.SeverityWarning, out.String())
	}
	if !strings.Contains(out.String(), "lakehouse schema") {
		t.Errorf("sweep output should still surface the finding, got: %s", out.String())
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
