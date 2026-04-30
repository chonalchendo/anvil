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
			"type": "issue", "title": title, "created": "2026-04-29",
			"updated": "2026-04-29", "status": "external", "project": project,
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
