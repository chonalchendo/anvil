package index

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestArtifactRowFromFrontmatter(t *testing.T) {
	fm := map[string]any{
		"type":    "issue",
		"id":      "demo.foo",
		"project": "demo",
		"status":  "open",
		"created": "2026-05-07",
		"updated": "2026-05-07",
	}
	got, err := ArtifactRowFromFrontmatter(fm, "/v/70-issues/demo.foo.md")
	if err != nil {
		t.Fatalf("ArtifactRowFromFrontmatter: %v", err)
	}
	want := ArtifactRow{
		ID: "demo.foo", Type: "issue", Status: "open",
		Project: "demo", Path: "/v/70-issues/demo.foo.md",
		Created: "2026-05-07", Updated: "2026-05-07",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("row mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_Scalar(t *testing.T) {
	fm := map[string]any{
		"type":      "issue",
		"id":        "demo.foo",
		"milestone": "[[milestone.demo.m1]]",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	want := []LinkRow{{Source: "demo.foo", Target: "demo.m1", Relation: "milestone", Anchor: ""}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_Array(t *testing.T) {
	fm := map[string]any{
		"type": "decision",
		"id":   "d1",
		"supersedes": []any{
			"[[decision.d0]]",
			"[[decision.d-1]]",
		},
	}
	got := LinkRowsFromFrontmatter("d1", fm)
	want := []LinkRow{
		// "d-1" < "d0" lexicographically ('-' < '0')
		{Source: "d1", Target: "d-1", Relation: "supersedes", Anchor: ""},
		{Source: "d1", Target: "d0", Relation: "supersedes", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromFrontmatter_IgnoresNonWikilinks(t *testing.T) {
	fm := map[string]any{
		"type":           "issue",
		"id":             "demo.foo",
		"external_links": []any{"https://example.com", "abc1234"},
		"severity":       "high",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows, got %v", got)
	}
}

// TestParseWikilink_PlanIssueEdge_BareID pins the indexer-side fix for
// anvil.anvil-create-plan-writes-issue-as-bare-id-indexer-drops-it-a: `anvil
// create plan` writes `issue:` as a bare id, so the indexer accepts both that
// and the wikilink form on typed-slot relations.
func TestParseWikilink_PlanIssueEdge_BareID(t *testing.T) {
	fm := map[string]any{
		"type":  "plan",
		"id":    "demo.foo-plan",
		"issue": "demo.foo",
	}
	got := LinkRowsFromFrontmatter("demo.foo-plan", fm)
	want := []LinkRow{{Source: "demo.foo-plan", Target: "demo.foo", Relation: "issue", Anchor: ""}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

// TestParseWikilink_PlanIssueEdge_WikilinkUnchanged guards the regression
// surface: existing plans that already use wikilink form continue to produce
// the same edge after the bare-id fallback is added.
func TestParseWikilink_PlanIssueEdge_WikilinkUnchanged(t *testing.T) {
	fm := map[string]any{
		"type":  "plan",
		"id":    "demo.foo-plan",
		"issue": "[[issue.demo.foo]]",
	}
	got := LinkRowsFromFrontmatter("demo.foo-plan", fm)
	want := []LinkRow{{Source: "demo.foo-plan", Target: "demo.foo", Relation: "issue", Anchor: ""}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

// TestParseWikilink_BareID_NonTypedSlotIgnored confirms the bare-id fallback
// is scoped to the typed-slot allowlist — random fields that happen to hold a
// dotted string (e.g. a hash or version) do not become spurious edges.
func TestParseWikilink_BareID_NonTypedSlotIgnored(t *testing.T) {
	fm := map[string]any{
		"type":     "issue",
		"id":       "demo.foo",
		"checksum": "demo.foo",
	}
	got := LinkRowsFromFrontmatter("demo.foo", fm)
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows for non-typed-slot field, got %v", got)
	}
}

func TestLinkRowsFromBody_DistinctTargets(t *testing.T) {
	body := "See [[issue.anvil.foo]] and [[learning.anvil.bar]] for context."
	got := LinkRowsFromBody("anvil.src", body)
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.bar", Relation: "body", Anchor: ""},
		{Source: "anvil.src", Target: "anvil.foo", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromBody_DedupWithinBody(t *testing.T) {
	body := "[[issue.anvil.foo]] is mentioned again: [[issue.anvil.foo]]."
	got := LinkRowsFromBody("anvil.src", body)
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.foo", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromBody_UnknownTypePrefixIgnored(t *testing.T) {
	body := "See [[bogustype.anvil.foo]] which is not a known Anvil type."
	got := LinkRowsFromBody("anvil.src", body)
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows for unknown type prefix, got %v", got)
	}
}

func TestLinkRowsFromBody_DeterministicOrdering(t *testing.T) {
	// Targets appear in reverse alphabetical order in body; output must be sorted.
	body := "[[issue.anvil.zzz]] then [[issue.anvil.aaa]]."
	got := LinkRowsFromBody("anvil.src", body)
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.aaa", Relation: "body", Anchor: ""},
		{Source: "anvil.src", Target: "anvil.zzz", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromBody_EmptyBody(t *testing.T) {
	got := LinkRowsFromBody("anvil.src", "")
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows for empty body, got %v", got)
	}
}

func TestLinkRowsFromBody_AliasedLink(t *testing.T) {
	body := "See [[issue.anvil.foo|the foo issue]] for details."
	got := LinkRowsFromBody("anvil.src", body)
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.foo", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

func TestLinkRowsFromBody_WhitespacePadded(t *testing.T) {
	body := "See [[ issue.anvil.bar ]] for details."
	got := LinkRowsFromBody("anvil.src", body)
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.bar", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

// TestLinkRowsFromBody_FencedWikilinkSkipped asserts that a wikilink inside a
// fenced code block is NOT emitted as a link row — it is illustrative text,
// not a live vault reference.
func TestLinkRowsFromBody_FencedWikilinkSkipped(t *testing.T) {
	body := "Prose wikilink: [[issue.anvil.real]].\n\n```bash\necho [[issue.anvil.ghost]]\n```\n"
	got := LinkRowsFromBody("anvil.src", body)
	// Only the prose link should appear; the fenced one must be absent.
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.real", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}

// TestLinkRowsFromBody_FencedOnlyNoRows asserts that a body containing only a
// fenced wikilink produces no rows at all.
func TestLinkRowsFromBody_FencedOnlyNoRows(t *testing.T) {
	body := "```\n[[issue.anvil.ghost]]\n```\n"
	got := LinkRowsFromBody("anvil.src", body)
	if len(got) != 0 {
		t.Fatalf("expected 0 link rows for fenced-only wikilink, got %v", got)
	}
}

// TestLinkRowsFromBody_TwoFencedBlocksProseInBetween exercises the non-greedy
// [\s\S]*? in fencedBlockRe: the first closing fence must not consume the
// second fenced block, and only the prose wikilink between them survives.
func TestLinkRowsFromBody_TwoFencedBlocksProseInBetween(t *testing.T) {
	body := "```bash\necho [[issue.anvil.ghost1]]\n```\n\nSee [[issue.anvil.real]] for context.\n\n```go\nfmt.Println([[issue.anvil.ghost2]])\n```\n"
	got := LinkRowsFromBody("anvil.src", body)
	want := []LinkRow{
		{Source: "anvil.src", Target: "anvil.real", Relation: "body", Anchor: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("link rows mismatch (-want +got):\n%s", diff)
	}
}
