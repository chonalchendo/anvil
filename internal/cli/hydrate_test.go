package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/output"
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

// writeHydrateIssueWithBody is writeHydrateIssue with a caller-controlled body,
// for pinning the `## Links` section walk (which lives in body text, not
// frontmatter).
func writeHydrateIssueWithBody(t *testing.T, vault, id string, links map[string]any, body string) {
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
	a := &core.Artifact{Path: filepath.Join(vault, "70-issues", id+".md"), FrontMatter: fm, Body: body}
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

// writeHydrateThread seeds a thread — a workspace artifact that must never
// enter hydrate's box even when named in an issue body's ## Links section
// (anvil.0240: governingBodyLinkTypes excludes thread/session/issue).
func writeHydrateThread(t *testing.T, vault, slug, body string) {
	t.Helper()
	dir := filepath.Join(vault, core.TypeThread.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable dirs
		t.Fatal(err)
	}
	id := slug
	a := &core.Artifact{
		Path: filepath.Join(dir, id+".md"),
		FrontMatter: map[string]any{
			"type": "thread", "title": id, "description": "fixture",
			"created": "2026-07-01", "updated": "2026-07-01", "status": "open", "tags": []any{},
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

	t.Run("closure ignores a thread named in the body ## Links section", func(t *testing.T) {
		// Pins anvil.0240 finding: an unfiltered body-## Links walk dragged a
		// workspace thread (891 lines on the live vault) into the box.
		// governingBodyLinkTypes must exclude thread even when the author
		// placed it in the explicit ## Links section.
		vault := setupVault(t)
		body := "## Problem\n\nfixture body.\n\n## Non-goals\n\n- none\n\n" +
			"## Verification\n\n### Direct\n\njust test\n\n### Indirect\n\nsmoke\n\n" +
			"## Links\n\n- [[convention.go-style]]\n- [[thread.foo-thread.0001-scratch]]\n"
		writeHydrateIssueWithBody(t, vault, "foo.i1", nil, body)
		writeHydrateConvention(t, vault, "go-style", "## Rules\n\nBODY_LINKS_SECTION_MARKER for the body-link hop.\n")
		writeHydrateThread(t, vault, "foo-thread.0001-scratch", "THREAD_BODY_MUST_NOT_ENTER_BOX_MARKER\n")

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if !strings.Contains(out, "BODY_LINKS_SECTION_MARKER") {
			t.Errorf("bundle missing body ## Links-section convention (governing type)\n%s", out)
		}
		if strings.Contains(out, "THREAD_BODY_MUST_NOT_ENTER_BOX_MARKER") {
			t.Errorf("bundle walked a body ## Links thread (workspace type must be excluded)\n%s", out)
		}
		// The dropped target must be stated, never silent (anvil.0240): the
		// manifest block reports it by name, and the node-header scrape
		// (`=== <type> <id> (status: <s>) ===`) must still find every node.
		wantSkipLine := "1 body ## Links target(s) not traversed (non-governing type): thread.foo-thread.0001-scratch"
		if !strings.Contains(out, wantSkipLine) {
			t.Errorf("manifest missing skipped-target line %q\n%s", wantSkipLine, out)
		}
		headerRe := regexp.MustCompile(`(?m)^=== \S+ \S+ \(status: [^)]+\) ===$`)
		if got := len(headerRe.FindAllString(out, -1)); got != 2 {
			t.Errorf("node-header scrape found %d headers, want 2 (issue + convention)\n%s", got, out)
		}
	})
}

// TestHydrateManifest pins anvil.0256: a convention four hops down the spine sits
// ~900 lines into a real closure, so a caller reading the head of the output
// concluded it was never returned. The manifest must name every node before the
// first body, in both emit modes.
func TestHydrateManifest(t *testing.T) {
	for _, mode := range []string{"full", "tldr"} {
		t.Run("manifest names every node ahead of the first body ("+mode+")", func(t *testing.T) {
			vault := setupVault(t)
			writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
			writeHydrateMilestone(t, vault, "foo.m1",
				map[string]any{"product_design": "[[product-design.foo]]"}, "milestone body\n")
			writeHydrateDesign(t, vault, "foo", core.TypeProductDesign,
				map[string]any{"related": []any{"[[convention.go-style]]"}}, "design body\n")
			writeHydrateConvention(t, vault, "go-style", "## Rules\n\nthe deepest node.\n")

			args := []string{"hydrate", "foo.i1"}
			if mode == "tldr" {
				args = append(args, "--tldr")
			}
			out, _, err := runCmd(t, newRootCmd(), args...)
			if err != nil {
				t.Fatalf("hydrate: %v", err)
			}

			firstBody := strings.Index(out, "=== issue issue.foo.i1 (status:")
			if firstBody < 0 {
				t.Fatalf("no issue body header in output\n%s", out)
			}
			head := out[:firstBody]
			if !strings.Contains(head, "=== hydrate manifest: 4 spine node(s)") {
				t.Errorf("manifest header missing or miscounted\n%s", head)
			}
			// Match the type and id as separate columns: asserting the rendered
			// `product-design foo` substring would pass only while the pad width
			// happens to equal len("product-design"), then fail claiming the node
			// is missing when the column widens.
			for _, want := range [][2]string{
				{"issue", "issue.foo.i1"},
				{"milestone", "milestone.foo.m1"},
				{"product-design", "foo"},
				{"convention", "convention.go-style"},
			} {
				line := regexp.MustCompile(`(?m)^\s+` + want[0] + `\s+` + regexp.QuoteMeta(want[1]) + ` \(`)
				if !line.MatchString(head) {
					t.Errorf("manifest omits %s %s ahead of the bodies\n%s", want[0], want[1], head)
				}
			}
		})
	}
}

// TestHydrateManifestEntriesAreNotHeaders pins the scrape contract: callers count
// bundle nodes with a `^=== <type> <id> (status: …) ===` grep, so the manifest's
// entry lines must not take that shape or every node would be counted twice.
func TestHydrateManifestEntriesAreNotHeaders(t *testing.T) {
	vault := setupVault(t)
	writeHydrateIssue(t, vault, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
	writeHydrateMilestone(t, vault, "foo.m1", nil, "milestone body\n")

	out, _, err := runCmd(t, newRootCmd(), "hydrate", "foo.i1")
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	got := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "=== ") && strings.Contains(line, "(status: ") {
			got++
		}
	}
	if got != 2 {
		t.Errorf("node-header scrape counted %d headers, want 2 (one per node)\n%s", got, out)
	}
}

// TestHydrateClippedBodyCarriesInlineMarker pins anvil.0258: a caller that pipes
// stdout and drops stderr must still be able to tell a body was truncated — the
// stderr-only BodyClipHint isn't enough on its own. It also pins the false-positive
// PR #397 flagged: hydrating an unclipped body must emit zero marker lines, since
// the marker phrase alone (without the anchored `=== body clipped` line start) is
// indistinguishable from artifact prose that happens to contain it.
func TestHydrateClippedBodyCarriesInlineMarker(t *testing.T) {
	longBody := strings.Repeat("line\n", showBodyLineCap+50)

	t.Run("clipped body carries the full marker line", func(t *testing.T) {
		v := setupVault(t)
		writeHydrateIssue(t, v, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, v, "foo.m1", nil, longBody)

		out, _, err := runCmd(t, newRootCmd(), "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		wantPath := filepath.Join(v, "85-milestones", "foo.m1.md")
		wantMarker := output.BodyClipMarker(showBodyLineCap+51, wantPath)
		found := false
		for _, line := range strings.Split(out, "\n") {
			if line == wantMarker {
				found = true
			}
		}
		if !found {
			t.Errorf("stdout missing exact marker line %q\n%s", wantMarker, out)
		}
	})

	t.Run("unclipped body carries zero marker lines", func(t *testing.T) {
		v := setupVault(t)
		writeHydrateIssue(t, v, "foo.i1", map[string]any{"milestone": "[[milestone.foo.m1]]"})
		writeHydrateMilestone(t, v, "foo.m1", nil, "short body\n")

		out, _, err := runCmd(t, newRootCmd(), "hydrate", "foo.i1")
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "=== body clipped") {
				t.Errorf("unclipped hydrate emitted a clip marker line %q\n%s", line, out)
			}
		}
	})
}
