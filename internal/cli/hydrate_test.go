package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeHydrateIssue seeds a schema-shaped issue whose frontmatter carries the
// given spine links (e.g. "milestone": "[[milestone.foo.m1]]").
func writeHydrateIssue(t *testing.T, vault, id string, links map[string]any) {
	t.Helper()
	fm := map[string]any{
		"type": "issue", "title": id, "description": "fixture description",
		"created": "2026-07-01", "updated": "2026-07-01",
		"status": "open", "project": "foo", "severity": "medium",
		"tags": []any{"domain/dev-tools"}, "goal": "fixture goal is done",
	}
	for k, v := range links {
		fm[k] = v
	}
	a := &core.Artifact{Path: filepath.Join(vault, "70-issues", id+".md"), FrontMatter: fm, Body: fixtureIssueBody}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// writeHydrateMilestone seeds a milestone with the given design links and a
// caller-controlled body so tests can assert the body text reaches the bundle.
func writeHydrateMilestone(t *testing.T, vault, id string, links map[string]any, body string) {
	t.Helper()
	fm := map[string]any{
		"type": "milestone", "title": id, "description": "fixture description",
		"created": "2026-07-01", "updated": "2026-07-01",
		"status": "planned", "project": "foo",
		"goal": "fixture milestone is done", "kind": "scoped",
	}
	for k, v := range links {
		fm[k] = v
	}
	a := &core.Artifact{Path: filepath.Join(vault, "85-milestones", id+".md"), FrontMatter: fm, Body: body}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// writeHydrateDesign seeds a product/system-design (prefix-retaining id) with a
// caller-controlled body. The design dirs are not scaffolded, so mkdir first.
func writeHydrateDesign(t *testing.T, vault, project string, typ core.Type, body string) {
	t.Helper()
	dir := filepath.Join(vault, typ.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable dirs
		t.Fatal(err)
	}
	id := string(typ) + "." + project
	a := &core.Artifact{
		Path: filepath.Join(dir, id+".md"),
		FrontMatter: map[string]any{
			"type": string(typ), "title": id, "description": "fixture description",
			"created": "2026-07-01", "status": "active", "project": project,
			"tags": []any{"type/" + string(typ)},
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// writeHydrateContract seeds a contract whose `## Code design` body links a
// convention (contracts link conventions from prose, not a frontmatter slot).
func writeHydrateContract(t *testing.T, vault, id, conventionTarget string) {
	t.Helper()
	dir := filepath.Join(vault, "75-contracts")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable dirs
		t.Fatal(err)
	}
	a := &core.Artifact{
		Path: filepath.Join(dir, id+".md"),
		FrontMatter: map[string]any{
			"type": "contract", "title": id, "description": "fixture",
			"created": "2026-07-01", "updated": "2026-07-01",
			"status": "active", "project": "foo", "kind": "data", "tags": []any{},
		},
		Body: "## Code design\n\nGoverned by [[" + conventionTarget + "]].\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// writeHydrateConvention seeds a convention (prefix-retaining id) with a
// caller-controlled body.
func writeHydrateConvention(t *testing.T, vault, slug, body string) {
	t.Helper()
	dir := filepath.Join(vault, core.TypeConvention.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable dirs
		t.Fatal(err)
	}
	id := string(core.TypeConvention) + "." + slug
	a := &core.Artifact{
		Path: filepath.Join(dir, id+".md"),
		FrontMatter: map[string]any{
			"type": "convention", "title": id, "description": "fixture",
			"created": "2026-07-01", "updated": "2026-07-01", "status": "active", "tags": []any{},
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestHydrate(t *testing.T) {
	t.Run("bundle carries linked milestone and design body text", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, vault, "foo.m1",
			map[string]any{"product_design": "[[product-design.foo]]"},
			"## Why now\n\nMILESTONE_MARKER_PHRASE spanning the spine.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeProductDesign,
			"## Vision\n\nPRODUCT_DESIGN_MARKER_PHRASE for the closure.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		for _, want := range []string{"MILESTONE_MARKER_PHRASE", "PRODUCT_DESIGN_MARKER_PHRASE"} {
			if !strings.Contains(out, want) {
				t.Errorf("bundle missing %q\n%s", want, out)
			}
		}
	})

	t.Run("closure walks the contract to its body-linked convention", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"related": []any{"[[contract.foo.boundaries]]"}})
		writeHydrateContract(t, vault, "foo.boundaries", "convention.go-style")
		writeHydrateConvention(t, vault, "go-style", "## Rules\n\nCONVENTION_MARKER for the contract hop.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if !strings.Contains(out, "CONVENTION_MARKER") {
			t.Errorf("bundle missing contract-linked convention body\n%s", out)
		}
	})

	t.Run("dangling spine target exits non-zero naming the broken edge", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.ghost]]"})

		cmd := newRootCmd()
		_, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err == nil {
			t.Fatal("expected non-zero exit for dangling milestone edge")
		}
		if !strings.Contains(err.Error(), "milestone.foo.ghost") {
			t.Errorf("error must name the broken edge target, got: %q", err.Error())
		}
	})

	t.Run("prefix-retaining design link resolves forward, not false-flagged", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, vault, "foo.m1",
			map[string]any{"system_design": "[[system-design.foo]]"},
			"## Why now\n\nmilestone body.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeSystemDesign,
			"## Architecture\n\nSYSTEM_DESIGN_MARKER for the forward-resolution check.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("prefix-retaining system-design link false-flagged as broken: %v", err)
		}
		if !strings.Contains(out, "SYSTEM_DESIGN_MARKER") {
			t.Errorf("bundle missing forward-resolved system-design body\n%s", out)
		}
	})
}
