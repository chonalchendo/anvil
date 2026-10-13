package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func TestCreate_DataModel_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "data-model", "--title", "Foo data model", "--description", "test description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "07-data-models", "foo.md"))
	if err != nil {
		t.Fatalf("expected file: %v", err)
	}
	if a.FrontMatter["type"] != "data-model" {
		t.Errorf("type = %v", a.FrontMatter["type"])
	}
	if err := schema.Validate("data-model", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestShow_DataModel_ResolvesByProject(t *testing.T) {
	vault := setupVault(t)
	writeFixtureDesign(t, vault, "foo", core.TypeDataModel, "Foo DM")

	out, _, err := runCmd(t, newRootCmd(), "show", "data-model", "foo")
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Foo DM") {
		t.Errorf("title missing:\n%s", out)
	}
}

// Warrant: a create that skips the section check writes a data model with no
// logical or physical layer that only validate later flags.
func TestCreate_DataModel_RefusesMissingSections(t *testing.T) {
	vault := setupVault(t)
	stdout, stderr, err := runCmd(t, newRootCmd(), "create", "data-model", "--project", "foo", "--title", "Foo DM", "--description", "d", "--body", "## Nope")
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("err = %v, want ErrSchemaInvalid", err)
	}
	if got := stdout + stderr + err.Error(); !strings.Contains(got, `missing required heading "## Domains"`) {
		t.Errorf("output lacks the missing-heading message:\n%s", got)
	}
	if _, statErr := os.Stat(filepath.Join(vault, "07-data-models", "foo.md")); !os.IsNotExist(statErr) {
		t.Errorf("refused create left a file behind: stat err = %v", statErr)
	}
}

// Warrant: a validate that skips the section check passes a data model with no
// logical or physical layer, so the model reads as complete when it is not.
func TestValidate_DataModel_SectionsAndDiagrams(t *testing.T) {
	vault := setupVault(t)
	path := writeFixtureDesign(t, vault, "foo", core.TypeDataModel, "Foo DM")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.FrontMatter["diagrams"] = []any{"ghost"}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", path})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("want validate failure\n%s", out.String())
	}
	for _, want := range []string{`missing required heading "## Logical"`, `diagram "ghost" has no file`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestDataModel_IDIsProjectAndWikilinkIsQualified(t *testing.T) {
	id, err := core.DeterministicID(core.TypeDataModel, core.IDInputs{Project: "foo"})
	if err != nil || id != "foo" {
		t.Fatalf("id = %q, err = %v", id, err)
	}
	if got := core.WikilinkTarget(core.TypeDataModel, id); got != "data-model.foo" {
		t.Errorf("wikilink target = %q", got)
	}
}
