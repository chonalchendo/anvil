package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeFixtureIssue(t *testing.T, vault, project, slug, title string) string {
	t.Helper()
	id := project + "." + slug
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "issue", "title": title, "description": "fixture description", "created": "2026-04-29",
			"updated": "2026-04-29", "status": "open", "project": project, "severity": "medium",
		},
		Body: "## Context\n\nfixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestShow_Text(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bar"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !bytes.Contains(out.Bytes(), []byte("title: \"Bar issue\"")) && !bytes.Contains(out.Bytes(), []byte("title: Bar issue")) {
		t.Errorf("title missing from output:\n%s", got)
	}
	if !bytes.Contains(out.Bytes(), []byte("fixture body")) {
		t.Errorf("body missing from output:\n%s", got)
	}
}

func TestShow_JSON(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bar", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["title"] != "Bar issue" {
		t.Errorf("title = %v", got["title"])
	}
	if _, ok := got["body"].(string); !ok {
		t.Errorf("body missing or not string: %v", got["body"])
	}
	if _, ok := got["path"].(string); !ok {
		t.Errorf("path missing or not string: %v", got["path"])
	}
}

func TestShow_MissingArtifact_ReturnsSentinel(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "nonexistent"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("err = %v, want ErrArtifactNotFound", err)
	}
}

func TestShow_UnknownType_Errors(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "bogus", "x"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestShowValidate_Issue_Clean(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "ok", "OK")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.ok", "--validate"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean validate, got %v\n%s", err, out.String())
	}
}

func TestShowValidate_Issue_DanglingMilestone(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "fixture description", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
			"milestone": "[[milestone.foo.ghost]]",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for dangling link")
	}
	if !errors.Is(err, ErrUnresolvedLinks) {
		t.Errorf("err = %v, want ErrUnresolvedLinks", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("milestone.foo.ghost")) {
		t.Errorf("output missing target name:\n%s", stderr.String())
	}
}

func TestShowValidate_Milestone_DanglingArrayEntry(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "85-milestones", "foo.m.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "milestone", "title": "M", "description": "fixture description", "created": "2026-04-29",
			"status": "planned", "project": "foo",
			"related": []any{"[[issue.foo.ghost]]"},
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "milestone", "foo.m", "--validate"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if !errors.Is(err, ErrUnresolvedLinks) {
		t.Errorf("err = %v, want ErrUnresolvedLinks", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("related[0]")) {
		t.Errorf("output missing field index:\n%s", stderr.String())
	}
}

func TestShowValidate_Issue_BadSchema(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "fixture description", "created": "2026-04-29",
			"status": "open", "project": "foo",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Errorf("err = %v, want ErrSchemaInvalid", err)
	}
}

func TestShowValidate_JSON(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "fixture description", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
			"milestone": "[[milestone.foo.ghost]]",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()

	// Assert wire-format keys are snake_case end-to-end: a struct with explicit
	// `json:"field"`/`json:"target"` tags would still accept CamelCase via
	// json.Unmarshal's case-insensitive matching, so we check the raw bytes.
	if !bytes.Contains(out.Bytes(), []byte(`"field":"milestone"`)) {
		t.Errorf("expected lowercase JSON key \"field\", got:\n%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"target":"milestone.foo.ghost"`)) {
		t.Errorf("expected lowercase JSON key \"target\", got:\n%s", out.String())
	}

	var got struct {
		SchemaOK        bool `json:"schema_ok"`
		UnresolvedLinks []struct {
			Field  string `json:"field"`
			Target string `json:"target"`
		} `json:"unresolved_links"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !got.SchemaOK {
		t.Errorf("schema_ok = false, want true")
	}
	if len(got.UnresolvedLinks) != 1 || got.UnresolvedLinks[0].Target != "milestone.foo.ghost" {
		t.Errorf("unresolved_links = %v", got.UnresolvedLinks)
	}
}
