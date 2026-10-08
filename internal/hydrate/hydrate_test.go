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

func TestAssemble(t *testing.T) {
	t.Run("walks the spine in order and dedups a convention on two rails", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{
			"milestone":        "[[milestone.foo.m1]]",
			"component_design": "[[component-design.foo.c1]]",
		}, "## Problem\n\nbody\n")
		writeArtifact(t, v, core.TypeMilestone, "milestone.foo.m1", map[string]any{"system_design": "[[system-design.foo]]"}, "m body\n")
		writeArtifact(t, v, core.TypeSystemDesign, "system-design.foo", map[string]any{"status": "active"}, "see [[convention.shared]]\n")
		writeArtifact(t, v, core.TypeComponentDesign, "component-design.foo.c1", map[string]any{}, "see [[convention.shared]]\n")
		writeArtifact(t, v, core.TypeConvention, "convention.shared", map[string]any{}, "rules\n")

		h, err := Assemble(v, "foo.i1")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"issue foo.i1", "milestone milestone.foo.m1", "system-design foo", "convention convention.shared", "component-design component-design.foo.c1"}
		if got := nodeIDs(h); !slices.Equal(got, want) {
			t.Errorf("nodes = %v, want %v", got, want)
		}
		if len(h.Broken) != 0 {
			t.Errorf("unexpected broken edges: %v", h.Broken)
		}
	})

	t.Run("dangling milestone records a broken edge and keeps walking", func(t *testing.T) {
		v := &core.Vault{Root: t.TempDir()}
		writeArtifact(t, v, core.TypeIssue, "foo.i1", map[string]any{"milestone": "[[milestone.foo.ghost]]"}, "b\n")

		h, err := Assemble(v, "foo.i1")
		if err != nil {
			t.Fatal(err)
		}
		want := BrokenEdge{Source: "issue foo.i1", Target: "milestone.foo.ghost"}
		if len(h.Broken) != 1 || h.Broken[0] != want {
			t.Errorf("broken = %v, want [%v]", h.Broken, want)
		}
	})

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
}
