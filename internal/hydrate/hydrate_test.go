package hydrate

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeArtifact(t *testing.T, v *core.Vault, typ core.Type, id string, fm map[string]any, body string) {
	t.Helper()
	fm["type"] = string(typ)
	path := filepath.Join(v.Root, typ.Dir(), id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func nodeIDs(h *Hydration) []string {
	ids := make([]string, 0, len(h.Nodes))
	for _, n := range h.Nodes {
		ids = append(ids, string(n.Type)+" "+n.ID)
	}
	return ids
}

const linksBody = "## Problem\n\nbody\n\n## Links\n\n"

func assemble(t *testing.T, v *core.Vault, id string) *Hydration {
	t.Helper()
	h, err := Assemble(v, id)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func wantNodes(t *testing.T, h *Hydration, want ...string) {
	t.Helper()
	if got := nodeIDs(h); !slices.Equal(got, want) {
		t.Errorf("nodes = %v, want %v", got, want)
	}
}

func wantBroken(t *testing.T, h *Hydration, want ...BrokenEdge) {
	t.Helper()
	if !slices.Equal(h.Broken, want) {
		t.Errorf("broken = %v, want %v", h.Broken, want)
	}
}

func cdesign(t *testing.T, v *core.Vault, id, conv string, fm map[string]any) {
	t.Helper()
	if fm == nil {
		fm = map[string]any{}
	}
	writeArtifact(t, v, core.TypeComponentDesign, id, fm, "## Code design\n\nGoverned by [["+conv+"]].\n")
}

func TestAssemble(t *testing.T) {
	t.Run("missing issue returns the typed not-found error", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		_, err := Assemble(v, "nope.9999")
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("err = %v, want *NotFoundError", err)
		}
		if nf.Input != "nope.9999" {
			t.Errorf("Input = %q, want nope.9999", nf.Input)
		}
	})

	t.Run("walks the component design to its body-linked convention", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"related": []any{"[[component-design.foo.boundaries]]"}}, "b\n")
		cdesign(t, v, "foo.boundaries", "convention.go-style", nil)
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")

		h := assemble(t, v, "foo.i1")
		wantNodes(t, h, "issue foo.i1", "component-design component-design.foo.boundaries", "convention convention.go-style")
		wantBroken(t, h)
	})

	t.Run("reaches a system design once via both rails and records a dangling slot", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"related": []any{"[[component-design.foo.boundaries]]"}}, "b\n")
		writeArtifact(t, v, core.TypeSystemDesign, "foo", map[string]any{}, "sys\n")
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")
		cdesign(t, v, "foo.boundaries", "convention.go-style", map[string]any{"system_design": "[[system-design.foo]]"})

		wantNodes(t, assemble(t, v, "foo.i1"),
			"issue foo.i1", "component-design component-design.foo.boundaries", "convention convention.go-style", "system-design foo")

		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{
			"milestone": "[[milestone.foo.m1]]",
			"related":   []any{"[[component-design.foo.boundaries]]"},
		}, "b\n")
		writeArtifact(t, v, core.TypeMilestone, "foo.m1", map[string]any{"system_design": "[[system-design.foo]]"}, "m\n")
		h := assemble(t, v, "foo.i1")
		wantNodes(t, h,
			"issue foo.i1", "milestone milestone.foo.m1", "system-design foo", "component-design component-design.foo.boundaries", "convention convention.go-style")

		cdesign(t, v, "foo.boundaries", "convention.go-style", map[string]any{"system_design": "[[system-design.ghost]]"})
		wantBroken(t, assemble(t, v, "foo.i1"), BrokenEdge{Source: "component design component-design.foo.boundaries", Target: "system-design.ghost"})
	})

	t.Run("a component design stays reachable when related also names a non-component target", func(t *testing.T) {
		// Pins anvil.0232: the component design sits AFTER the non-matching element,
		// so a break-at-first-miss regression in LinkTargetsOfType turns this red.
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{
			"related": []any{"[[system-design.foo]]", "[[component-design.foo.boundaries]]"},
		}, "b\n")
		cdesign(t, v, "foo.boundaries", "convention.go-style", nil)
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")
		writeArtifact(t, v, core.TypeSystemDesign, "foo", map[string]any{}, "sys\n")

		h := assemble(t, v, "foo.i1")
		if !slices.Contains(nodeIDs(h), "component-design component-design.foo.boundaries") {
			t.Errorf("component design dropped: %v", nodeIDs(h))
		}
		wantBroken(t, h)
	})

	t.Run("resolves a component design minted with the type-prefixed filename", func(t *testing.T) {
		// Pins anvil.0232: a probe regression to the bare-only shape reports a broken edge.
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"related": []any{"[[component-design.foo.boundaries]]"}}, "b\n")
		cdesign(t, v, "component-design.foo.boundaries", "convention.go-style", nil)
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")

		h := assemble(t, v, "foo.i1")
		wantNodes(t, h, "issue foo.i1", "component-design component-design.foo.boundaries", "convention convention.go-style")
		wantBroken(t, h)
	})

	t.Run("walks a design to its linked convention with no component design", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"}, "b\n")
		writeArtifact(t, v, core.TypeMilestone, "foo.m1", map[string]any{"product_design": "[[product-design.foo]]"}, "m\n")
		writeArtifact(t, v, core.TypeProductDesign, "foo", map[string]any{"related": []any{"[[convention.go-style]]"}}, "d\n")
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")

		wantNodes(t, assemble(t, v, "foo.i1"),
			"issue foo.i1", "milestone milestone.foo.m1", "product-design foo", "convention convention.go-style")
	})

	t.Run("a convention on both rails enters the closure once", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{
			"milestone": "[[milestone.foo.m1]]",
			"related":   []any{"[[component-design.foo.boundaries]]"},
		}, "b\n")
		writeArtifact(t, v, core.TypeMilestone, "foo.m1", map[string]any{"product_design": "[[product-design.foo]]"}, "m\n")
		writeArtifact(t, v, core.TypeProductDesign, "foo", map[string]any{"related": []any{"[[convention.go-style]]"}}, "d\n")
		cdesign(t, v, "foo.boundaries", "convention.go-style", nil)
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")

		wantNodes(t, assemble(t, v, "foo.i1"),
			"issue foo.i1", "milestone milestone.foo.m1", "product-design foo", "convention convention.go-style",
			"component-design component-design.foo.boundaries")
	})

	t.Run("a bare-id design link resolves forward, not flagged broken", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"}, "b\n")
		writeArtifact(t, v, core.TypeMilestone, "foo.m1", map[string]any{"system_design": "[[system-design.foo]]"}, "m\n")
		writeArtifact(t, v, core.TypeSystemDesign, "foo", map[string]any{}, "sys\n")

		h := assemble(t, v, "foo.i1")
		wantNodes(t, h, "issue foo.i1", "milestone milestone.foo.m1", "system-design foo")
		wantBroken(t, h)
	})

	t.Run("walks a wikilink in the issue body ## Links section", func(t *testing.T) {
		// Pins anvil.0240: a convention named only in the body ## Links section.
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{}, linksBody+"- [[convention.go-style]]\n")
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")

		h := assemble(t, v, "foo.i1")
		wantNodes(t, h, "issue foo.i1", "convention convention.go-style")
		wantBroken(t, h)
	})

	t.Run("a dangling governing-type body ## Links target is a broken edge", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{}, linksBody+"- [[convention.ghost]]\n")

		wantBroken(t, assemble(t, v, "foo.i1"), BrokenEdge{Source: "issue foo.i1", Target: "convention.ghost"})
	})

	t.Run("a non-governing body ## Links target is skipped and named", func(t *testing.T) {
		// Pins anvil.0240: an unfiltered walk dragged a workspace thread into the box.
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{},
			linksBody+"- [[convention.go-style]]\n- [[thread.foo-thread.0001-scratch]]\n")
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")
		writeArtifact(t, v, core.TypeThread, "foo-thread.0001-scratch", map[string]any{}, "scratch\n")

		h := assemble(t, v, "foo.i1")
		wantNodes(t, h, "issue foo.i1", "convention convention.go-style")
		if want := []string{"thread.foo-thread.0001-scratch"}; !slices.Equal(h.SkippedBodyLinks, want) {
			t.Errorf("SkippedBodyLinks = %v, want %v", h.SkippedBodyLinks, want)
		}
	})

	t.Run("a node on both the spine and the body ## Links enters once", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"related": []any{"[[component-design.foo.boundaries]]"}},
			linksBody+"- [[component-design.foo.boundaries]]\n")
		cdesign(t, v, "foo.boundaries", "convention.go-style", nil)
		writeArtifact(t, v, core.TypeConvention, "convention.go-style", map[string]any{}, "rules\n")

		wantNodes(t, assemble(t, v, "foo.i1"),
			"issue foo.i1", "component-design component-design.foo.boundaries", "convention convention.go-style")
	})
}
