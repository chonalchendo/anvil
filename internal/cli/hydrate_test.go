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

// writeHydrateDesign seeds a product/system-design (prefix-retaining id) with the
// given links (e.g. "related": []any{"[[convention.go-style]]"}) and a
// caller-controlled body. The design dirs are not scaffolded, so mkdir first.
func writeHydrateDesign(t *testing.T, vault, project string, typ core.Type, links map[string]any, body string) {
	t.Helper()
	dir := filepath.Join(vault, typ.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable dirs
		t.Fatal(err)
	}
	id := project
	fm := map[string]any{
		"type": string(typ), "title": string(typ) + "." + project, "description": "fixture description",
		"created": "2026-07-01", "status": "active", "project": project,
		"tags": []any{"type/" + string(typ)},
	}
	for k, v := range links {
		fm[k] = v
	}
	a := &core.Artifact{
		Path:        filepath.Join(dir, id+".md"),
		FrontMatter: fm,
		Body:        body,
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

// writeHydrateLearning seeds a learning (id is the bare slug — learnings drop the
// type prefix on disk) with a caller-controlled body so tests can assert its
// `## TL;DR` reaches the digest.
func writeHydrateLearning(t *testing.T, vault, slug, body string) {
	t.Helper()
	dir := filepath.Join(vault, core.TypeLearning.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable dirs
		t.Fatal(err)
	}
	id := slug
	a := &core.Artifact{
		Path: filepath.Join(dir, id+".md"),
		FrontMatter: map[string]any{
			"type": "learning", "title": id, "description": "fixture",
			"created": "2026-07-01", "updated": "2026-07-01", "status": "verified", "tags": []any{},
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
		writeHydrateDesign(t, vault, "foo", core.TypeProductDesign, nil,
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

	t.Run("closure resolves a contract link when its related list also names a non-contract target", func(t *testing.T) {
		// Pins anvil.0232: a real issue's `related:` frontmatter mixed a contract
		// wikilink with a system-design wikilink in one list; the contract was
		// observed dropped from the closure while every other node resolved.
		// The contract sits AFTER the non-matching element so the walk must scan
		// past it — contract-first would stay green under a break-at-first-miss
		// regression in linkTargetsOfType.
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{
			"related": []any{"[[system-design.foo]]", "[[contract.foo.boundaries]]"},
		})
		writeHydrateContract(t, vault, "foo.boundaries", "convention.go-style")
		writeHydrateConvention(t, vault, "go-style", "## Rules\n\nCONTRACT_ALONGSIDE_DESIGN_MARKER.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeSystemDesign, nil, "## System\n\nsystem design body.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if !strings.Contains(out, "=== contract contract.foo.boundaries") {
			t.Errorf("bundle missing contract linked alongside a non-contract related target\n%s", out)
		}
		if !strings.Contains(out, "CONTRACT_ALONGSIDE_DESIGN_MARKER") {
			t.Errorf("bundle missing contract-linked convention body\n%s", out)
		}
	})

	t.Run("closure resolves a contract minted with the type-prefixed filename", func(t *testing.T) {
		// Pins anvil.0232's confirmed drop direction: a contract minted with the
		// type-prefixed filename (anvil.0202) must resolve from its canonical
		// wikilink. If core.ArtifactBasename's probe regresses to the pre-#354
		// bare-only shape that caused the 2026-07-30 drops, hydrate exits with
		// "1 broken spine edge(s)" and this goes red.
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"related": []any{"[[contract.foo.boundaries]]"}})
		writeHydrateContract(t, vault, "contract.foo.boundaries", "convention.go-style")
		writeHydrateConvention(t, vault, "go-style", "## Rules\n\nconvention body.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if !strings.Contains(out, "=== contract contract.foo.boundaries") {
			t.Errorf("bundle missing prefixed-filename contract\n%s", out)
		}
	})

	t.Run("closure walks the design to its linked convention with no contract rail", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, vault, "foo.m1",
			map[string]any{"product_design": "[[product-design.foo]]"},
			"## Why now\n\nmilestone body.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeProductDesign,
			map[string]any{"related": []any{"[[convention.go-style]]"}},
			"## Vision\n\ndesign body.\n")
		writeHydrateConvention(t, vault, "go-style", "## Rules\n\nDESIGN_CONVENTION_MARKER for the design hop.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if !strings.Contains(out, "DESIGN_CONVENTION_MARKER") {
			t.Errorf("bundle missing design-linked convention body\n%s", out)
		}
	})

	t.Run("a convention on both rails enters the closure once", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{
			"milestone": "[[milestone.foo.m1]]",
			"related":   []any{"[[contract.foo.boundaries]]"},
		})
		writeHydrateMilestone(t, vault, "foo.m1",
			map[string]any{"product_design": "[[product-design.foo]]"},
			"## Why now\n\nmilestone body.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeProductDesign,
			map[string]any{"related": []any{"[[convention.go-style]]"}},
			"## Vision\n\ndesign body.\n")
		writeHydrateContract(t, vault, "foo.boundaries", "convention.go-style")
		writeHydrateConvention(t, vault, "go-style", "## Rules\n\nshared by both rails.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if got := strings.Count(out, "=== convention convention.go-style"); got != 1 {
			t.Errorf("convention reachable by both rails emitted %d times, want 1\n%s", got, out)
		}
	})

	t.Run("--tldr emits frontmatter and TL;DR, suppressing full bodies", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{
			"milestone": "[[milestone.foo.m1]]",
			"related":   []any{"[[learning.gotcha]]"},
		})
		writeHydrateMilestone(t, vault, "foo.m1", map[string]any{},
			"## Why now\n\nMILESTONE_BODY_MARKER that must not appear in the digest.\n")
		writeHydrateLearning(t, vault, "gotcha",
			"## TL;DR\n\nLEARNING_TLDR_MARKER the digest keeps.\n\n## Evidence\n\nEVIDENCE_MARKER the digest drops.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1", "--tldr")
		if err != nil {
			t.Fatalf("hydrate --tldr: %v", err)
		}
		// Headers and spine order preserved; frontmatter (goal) carries the summary.
		for _, want := range []string{"=== issue issue.foo.i1", "fixture goal is done", "LEARNING_TLDR_MARKER"} {
			if !strings.Contains(out, want) {
				t.Errorf("digest missing %q\n%s", want, out)
			}
		}
		// Full bodies and below-TL;DR learning sections are dropped.
		for _, unwanted := range []string{"MILESTONE_BODY_MARKER", "EVIDENCE_MARKER"} {
			if strings.Contains(out, unwanted) {
				t.Errorf("digest leaked full-body text %q\n%s", unwanted, out)
			}
		}
	})

	t.Run("--tldr ignores inline mentions, empty sections, and fenced headings", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{
			"related": []any{"[[learning.l-inline]]", "[[learning.l-empty]]", "[[learning.l-fenced]]"},
		})
		// A prose mention of the heading is not a section.
		writeHydrateLearning(t, vault, "l-inline",
			"Convention prose: open every learning with a ## TL;DR INLINE_MENTION_MARKER heading.\n\n## Notes\n\nbody text.\n")
		// An empty section yields no digest — not a bare heading, and never the
		// following section riding along.
		writeHydrateLearning(t, vault, "l-empty",
			"## TL;DR\n\n## Evidence\n\nEMPTY_SECTION_MARKER text.\n")
		// A heading quoted inside a fenced code block is not a section.
		writeHydrateLearning(t, vault, "l-fenced",
			"```\n## TL;DR\nFENCED_LEAK_MARKER inside fence\n```\n\n## Context\n\nbody text.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1", "--tldr")
		if err != nil {
			t.Fatalf("hydrate --tldr: %v", err)
		}
		// None of the three carries a real TL;DR, so no digest heading at all.
		for _, unwanted := range []string{"## TL;DR", "INLINE_MENTION_MARKER", "## Evidence", "EMPTY_SECTION_MARKER", "FENCED_LEAK_MARKER"} {
			if strings.Contains(out, unwanted) {
				t.Errorf("digest leaked %q from a body with no real TL;DR section\n%s", unwanted, out)
			}
		}
	})

	t.Run("empty-bodied node header is marked empty", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, vault, "foo.m1",
			map[string]any{"product_design": "[[product-design.foo]]"},
			"## Why now\n\nMILESTONE_BODY_HAS_CONTENT.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeProductDesign, nil, "")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if !strings.Contains(out, "=== product-design foo (status: active, empty) ===") {
			t.Errorf("empty-bodied design node header missing the `empty` marker\n%s", out)
		}
		if strings.Contains(out, "=== milestone milestone.foo.m1 (status: planned, empty)") {
			t.Errorf("milestone with real body wrongly marked empty\n%s", out)
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

	t.Run("bare-id design link resolves forward, not false-flagged", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, vault, "foo.m1",
			map[string]any{"system_design": "[[system-design.foo]]"},
			"## Why now\n\nmilestone body.\n")
		writeHydrateDesign(t, vault, "foo", core.TypeSystemDesign, nil,
			"## Architecture\n\nSYSTEM_DESIGN_MARKER for the forward-resolution check.\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("bare-id system-design link false-flagged as broken: %v", err)
		}
		if !strings.Contains(out, "SYSTEM_DESIGN_MARKER") {
			t.Errorf("bundle missing forward-resolved system-design body\n%s", out)
		}
	})
}
