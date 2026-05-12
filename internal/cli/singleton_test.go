package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeFixtureSingleton writes a singleton artifact (product-design or
// system-design) to <vault>/05-projects/<project>/<type>.md.
func writeFixtureSingleton(t *testing.T, vault, project string, typ core.Type, title string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(vault, "05-projects", project), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "05-projects", project, string(typ)+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": string(typ), "title": title, "description": "fixture description",
			"created": "2026-05-12", "status": "active", "project": project,
			"tags": []any{"type/" + string(typ)},
		},
		Body: "## Context\n\nfixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestList_ProductDesign_ReturnsSingletons(t *testing.T) {
	vault := setupVault(t)
	writeFixtureSingleton(t, vault, "foo", core.TypeProductDesign, "Foo PD")
	writeFixtureSingleton(t, vault, "bar", core.TypeProductDesign, "Bar PD")
	// system-design in the same dir must not leak into product-design output.
	writeFixtureSingleton(t, vault, "foo", core.TypeSystemDesign, "Foo SD")

	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "product-design", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 2 {
		t.Fatalf("total=%d, want 2; items=%+v", env.Total, env.Items)
	}
	ids := map[string]bool{}
	for _, it := range env.Items {
		ids[it.ID] = true
		if it.Type != "product-design" {
			t.Errorf("type=%q, want product-design", it.Type)
		}
		if it.Project == "" {
			t.Errorf("project empty for %+v", it)
		}
	}
	if !ids["bar"] || !ids["foo"] {
		t.Errorf("missing ids: %v", ids)
	}
}

func TestList_SystemDesign_ReturnsSingletons(t *testing.T) {
	vault := setupVault(t)
	writeFixtureSingleton(t, vault, "foo", core.TypeSystemDesign, "Foo SD")
	writeFixtureSingleton(t, vault, "foo", core.TypeProductDesign, "Foo PD")

	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "system-design", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 1 {
		t.Fatalf("total=%d, want 1; items=%+v", env.Total, env.Items)
	}
	if env.Items[0].Type != "system-design" || env.Items[0].ID != "foo" {
		t.Errorf("got %+v, want type=system-design id=foo", env.Items[0])
	}
}

func TestShow_ProductDesign_ResolvesByProject(t *testing.T) {
	vault := setupVault(t)
	writeFixtureSingleton(t, vault, "foo", core.TypeProductDesign, "Foo PD")

	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "show", "product-design", "foo")
	if err != nil {
		t.Fatalf("show by project: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "Foo PD") {
		t.Errorf("title missing:\n%s", out)
	}
}

func TestShow_ProductDesign_ResolvesByQualifiedID(t *testing.T) {
	vault := setupVault(t)
	writeFixtureSingleton(t, vault, "foo", core.TypeProductDesign, "Foo PD")

	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "show", "product-design", "product-design.foo")
	if err != nil {
		t.Fatalf("show by qualified id: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "Foo PD") {
		t.Errorf("title missing:\n%s", out)
	}
}

func TestShow_SystemDesign_ResolvesByProject(t *testing.T) {
	vault := setupVault(t)
	writeFixtureSingleton(t, vault, "foo", core.TypeSystemDesign, "Foo SD")

	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "show", "system-design", "foo")
	if err != nil {
		t.Fatalf("show by project: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "Foo SD") {
		t.Errorf("title missing:\n%s", out)
	}
}

func TestValidate_DetectsBadSingleton(t *testing.T) {
	vault := setupVault(t)
	// Plant a singleton with invalid status — should be caught by validate.
	if err := os.MkdirAll(filepath.Join(vault, "05-projects", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := &core.Artifact{
		Path: filepath.Join(vault, "05-projects", "foo", "product-design.md"),
		FrontMatter: map[string]any{
			"type": "product-design", "title": "x", "created": "2026-04-29",
			"description": "x", "status": "totally-bogus",
		},
		Body: "",
	}
	if err := bad.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	if err := cmd.Execute(); err == nil {
		t.Error("expected validation error for bad singleton, got nil")
	}
}

func TestCreate_HelpEnumeratesTypes(t *testing.T) {
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "create", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{
		"inbox", "issue", "plan", "milestone", "decision", "learning",
		"thread", "sweep", "session", "product-design", "system-design",
	} {
		if !strings.Contains(out, typ) {
			t.Errorf("create --help missing type %q:\n%s", typ, out)
		}
	}
}
