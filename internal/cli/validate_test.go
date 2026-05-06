package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestValidate_GoodVault(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	// Add one valid issue.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "good", "--description", "test description", "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	val := newRootCmd()
	val.SetArgs([]string{"validate", vault})
	if err := val.Execute(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidate_BadFrontmatter(t *testing.T) {
	vault := setupVault(t)

	// Plant an issue with invalid status.
	bad := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.bad.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "created": "2026-04-29",
			"status": "totally-bogus",
		},
		Body: "",
	}
	if err := bad.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected validation error")
	}
}

func TestValidate_DefaultsToAnvilVault(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate empty vault failed: %v", err)
	}
	_ = os.Remove // silence unused if not needed
}

func TestValidate_Learning_BodyShape(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "learning", "--title", "X", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "20-learnings", "x.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.Body = "\n## TL;DR\nclaim\n\n## Caveats\nlimit\n"
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd = newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	out.Reset()
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected validate to fail")
	}
	if !strings.Contains(errOut.String(), "Evidence") {
		t.Errorf("expected Evidence in stderr, got %q", errOut.String())
	}
}

func TestValidate_JSON_StructuredErrors(t *testing.T) {
	vault := setupVault(t)
	bad := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.bad.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d",
			"created": "2026-05-05", "status": "raw-input",
			"project": "foo", "severity": "low",
		},
	}
	if err := bad.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	var got []map[string]any
	if jerr := json.Unmarshal(out.Bytes(), &got); jerr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jerr, out.String())
	}
	if len(got) == 0 {
		t.Fatal("expected at least one error")
	}
	var found map[string]any
	for _, e := range got {
		if e["field"] == "status" {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatalf("expected error for field=status, got %v", got)
	}
	if found["code"] != "enum_violation" {
		t.Errorf("code=%v want enum_violation", found["code"])
	}
	if found["got"] != "raw-input" {
		t.Errorf("got=%v want raw-input", found["got"])
	}
	if found["path"] == "" {
		t.Error("path missing")
	}
	if _, ok := found["expected"].([]any); !ok {
		t.Errorf("expected should be array, got %T", found["expected"])
	}
}

func TestValidate_JSON_MissingRequired(t *testing.T) {
	vault := setupVault(t)
	bad := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.miss.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x",
			"created": "2026-05-05", "status": "open",
			"project": "foo", "severity": "low",
		},
	}
	if err := bad.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var found map[string]any
	for _, e := range got {
		if e["code"] == "missing_required" && e["field"] == "description" {
			found = e
		}
	}
	if found == nil {
		t.Errorf("expected missing_required for description, got %v", got)
	}
}

func TestValidate_MissingRequiredFacet_Issue(t *testing.T) {
	vault := setupVault(t)
	bad := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.x.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "y",
			"created": "2026-05-06", "status": "open",
			"project": "anvil", "severity": "low",
			"tags": []any{"pattern/idempotency"},
		},
	}
	if err := bad.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", "--json", vault})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()

	var failures []map[string]any
	if err := json.Unmarshal(out.Bytes(), &failures); err != nil {
		t.Fatalf("parse json: %v\n%s", err, out.String())
	}
	found := false
	for _, f := range failures {
		if f["code"] == "missing_required_facet" && f["field"] == "tags" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing_required_facet on tags, got %+v", failures)
	}
}

func TestValidate_TextMode_BlocksSeparated(t *testing.T) {
	vault := setupVault(t)
	bad1 := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.a.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d",
			"created": "2026-05-05", "status": "bogus",
			"project": "foo", "severity": "low",
		},
	}
	bad1.Save()
	bad2 := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.b.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d",
			"created": "2026-05-05", "status": "open",
			"project": "foo", "severity": "yikes",
		},
	}
	bad2.Save()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	_ = cmd.Execute()
	if !strings.Contains(errOut.String(), "\n\n") {
		t.Errorf("expected blank line between blocks, got:\n%s", errOut.String())
	}
}
