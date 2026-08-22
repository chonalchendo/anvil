package core

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeBlankIssue(t *testing.T, v *Vault, id string) {
	t.Helper()
	p := filepath.Join(v.Root, TypeIssue.Dir(), id+".md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("---\ntype: issue\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
}

func TestResolveLinks_AllPresent(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.x")
	fm := map[string]any{
		"milestone": "[[milestone.anvil.cli-substrate]]",
		"related":   []any{"[[issue.anvil.x]]"},
	}
	mp := filepath.Join(v.Root, TypeMilestone.Dir(), "anvil.cli-substrate.md")
	_ = os.MkdirAll(filepath.Dir(mp), 0o755)                           //nolint:gosec // 0755 is correct for directories that must be traversable
	_ = os.WriteFile(mp, []byte("---\ntype: milestone\n---\n"), 0o644) //nolint:gosec // 0644 is correct for config/data files readable by owner and group

	got := ResolveLinks(v, fm)
	if len(got) != 0 {
		t.Errorf("expected 0 unresolved, got %v", got)
	}
}

func TestResolveLinks_DanglingScalar(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{"milestone": "[[milestone.anvil.ghost]]"}
	got := ResolveLinks(v, fm)
	want := []UnresolvedLink{{Field: "milestone", Target: "milestone.anvil.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveLinks_DanglingArrayWithIndex(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.real")
	fm := map[string]any{
		"related": []any{
			"[[issue.anvil.real]]",
			"[[issue.anvil.ghost]]",
		},
	}
	got := ResolveLinks(v, fm)
	want := []UnresolvedLink{{Field: "related[1]", Target: "issue.anvil.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveLinks_NonWikilinkIgnored(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{
		"title":  "Plain string, not a wikilink",
		"status": "open",
	}
	if got := ResolveLinks(v, fm); len(got) != 0 {
		t.Errorf("expected no unresolved, got %v", got)
	}
}

// TestResolveLinks_ProseFieldQuotedLinkIgnored pins the declared-slot
// restriction: a wikilink quoted inside a prose field (title, acceptance) is
// commentary, not a graph edge, and must not be reported even when dangling.
func TestResolveLinks_ProseFieldQuotedLinkIgnored(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{
		"title":      "quoting a broken [[convention.ghost]] in prose",
		"acceptance": []any{"loads an issue's [[system-design.ghost]] links at issue-start"},
	}
	if got := ResolveLinks(v, fm); len(got) != 0 {
		t.Errorf("prose-quoted wikilinks must be ignored, got %v", got)
	}
}

func TestResolveLinks_UnknownTypePrefix_Ignored(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{"author": "[[people.alice]]"}
	if got := ResolveLinks(v, fm); len(got) != 0 {
		t.Errorf("unknown-prefix tokens should be ignored, got %v", got)
	}
}

func TestResolveLinks_CrossProject(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "dbt-warehouse.add-revenue-model")
	fm := map[string]any{
		"depends_on": []any{
			"[[issue.dbt-warehouse.add-revenue-model]]",
			"[[issue.airflow-pipelines.ghost]]",
		},
	}
	got := ResolveLinks(v, fm)
	want := []UnresolvedLink{{Field: "depends_on[1]", Target: "issue.airflow-pipelines.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveLinks_Stable(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{
		"milestone": "[[milestone.anvil.ghost]]",
		"related":   []any{"[[issue.anvil.ghost]]"},
	}
	a := ResolveLinks(v, fm)
	b := ResolveLinks(v, fm)
	sort.Slice(a, func(i, j int) bool { return a[i].Field < a[j].Field })
	sort.Slice(b, func(i, j int) bool { return b[i].Field < b[j].Field })
	if !reflect.DeepEqual(a, b) {
		t.Errorf("non-deterministic: %v vs %v", a, b)
	}
}

// TestBodyWikilinkTargetsOfType covers the contract→convention body rail:
// body `[[convention.X]]` links are surfaced (full target, deduped, first-seen
// order), other types and fenced/aliased links are filtered as the indexer does.
func TestBodyWikilinkTargetsOfType(t *testing.T) {
	body := "Style: [[convention.python]] and [[convention.sql|SQL]].\n" +
		"Unrelated: [[issue.anvil.foo]].\n" +
		"Repeat: [[convention.python]].\n" +
		"```\n[[convention.fenced]]\n```\n"
	got := BodyWikilinkTargetsOfType(body, TypeConvention)
	want := []string{"convention.python", "convention.sql"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestBodyLinksSectionTargets pins anvil.0240: only wikilinks inside the
// `## Links` section are returned, filtered to governingBodyLinkTypes; a
// prose mention elsewhere in the body, a fenced illustration, and an unknown
// type prefix are all excluded — and so is a workspace/history type (thread,
// sibling issue) placed inside the section itself, since a governing-type
// link entering the box is only half the regression this pins.
func TestBodyLinksSectionTargets(t *testing.T) {
	body := "## Problem\n\nSee [[convention.prose-mention]] in passing.\n\n" +
		"## Links\n\n- [[convention.go-style]]\n- [[contract.foo.boundaries|Boundaries]]\n" +
		"- [[convention.go-style]]\n- [[project.not-a-real-type]]\n" +
		"- [[thread.foo-thread.0001-scratch]]\n- [[issue.anvil.0001.sibling]]\n" +
		"```\n[[convention.fenced]]\n```\n\n" +
		"## Trailing\n\nafter the section, ignored: [[convention.after]].\n"
	got := BodyLinksSectionTargets(body)
	want := []string{"convention.go-style", "contract.foo.boundaries"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestBodyLinksSectionTargets_NoSection asserts a body with no `## Links`
// heading returns nil rather than falling back to a full-body scan.
func TestBodyLinksSectionTargets_NoSection(t *testing.T) {
	body := "## Problem\n\nSee [[convention.go-style]].\n"
	if got := BodyLinksSectionTargets(body); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// TestResolveBodyLinks_FencedWikilinkSkipped asserts that a wikilink inside a
// fenced code block is not flagged as unresolved — it is illustrative text,
// not a live vault reference.
func TestResolveBodyLinks_FencedWikilinkSkipped(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.real")
	body := "Prose link: [[issue.anvil.real]].\n\n```bash\necho [[issue.anvil.ghost]]\n```\n"
	got := ResolveBodyLinks(v, body)
	// The fenced ghost link must not appear; the prose link resolves.
	if len(got) != 0 {
		t.Errorf("expected 0 unresolved, got %v", got)
	}
}

// TestResolveBodyLinks_ProseWikilinkStillValidated asserts that an unresolved
// wikilink in prose (not inside a fence) is still reported as unresolved.
func TestResolveBodyLinks_ProseWikilinkStillValidated(t *testing.T) {
	v := newScaffolded(t)
	body := "See [[issue.anvil.ghost]] for context.\n"
	got := ResolveBodyLinks(v, body)
	want := []UnresolvedLink{{Field: "body", Target: "issue.anvil.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveBodyLinks_PlaceholderWikilinkLiteral asserts that a [[...]] whose
// inner target contains id-illegal chars (<, >, or whitespace) is treated as
// literal text and not flagged as an unresolved link. Such targets can never be
// real artifact ids, so they are documentation placeholders, not live links.
func TestResolveBodyLinks_PlaceholderWikilinkLiteral(t *testing.T) {
	v := newScaffolded(t)
	cases := []struct {
		name string
		body string
	}{
		{"angle bracket metavar", "Illustration: [[milestone.<project>.<slug>]] is a placeholder."},
		{"space in target", "See [[some thing with spaces]] here."},
		{"leading angle bracket", "Use [[<type>.<project>.<id>]] syntax."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBodyLinks(v, tc.body)
			if len(got) != 0 {
				t.Errorf("expected 0 unresolved for placeholder wikilink, got %v", got)
			}
		})
	}
}

// TestResolveLinks_DesignDocPresent asserts that a [[product-design.<project>]]
// or [[system-design.<project>[.<shard>]]] wikilink resolves under the per-type
// flat layout. Design ids are bare (no type prefix), so the on-disk id is the
// wikilink target's tail (e.g. burgh, anvil.build).
func TestResolveLinks_DesignDocPresent(t *testing.T) {
	v := newScaffolded(t)
	files := map[Type][]string{
		TypeProductDesign: {"burgh"},
		TypeSystemDesign:  {"burgh", "anvil.build"},
	}
	for typ, ids := range files {
		for _, id := range ids {
			p := typ.Path(v.Root, id)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("---\ntype: "+string(typ)+"\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
				t.Fatal(err)
			}
		}
	}
	fm := map[string]any{
		"product_design": "[[product-design.burgh]]",
		"system_design":  "[[system-design.burgh]]",
		"shard_design":   "[[system-design.anvil.build]]",
	}
	got := ResolveLinks(v, fm)
	if len(got) != 0 {
		t.Errorf("expected 0 unresolved design-doc links, got %v", got)
	}
}

// TestResolveLinks_SingletonDesignDocMissing asserts that a dangling
// [[product-design.<project>]] wikilink (no file present) is still reported.
func TestResolveLinks_SingletonDesignDocMissing(t *testing.T) {
	v := newScaffolded(t)
	fm := map[string]any{
		"product_design": "[[product-design.ghost]]",
	}
	got := ResolveLinks(v, fm)
	want := []UnresolvedLink{{Field: "product_design", Target: "product-design.ghost"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveBodyLinks_BareProjectSlugFlagged asserts that a body wikilink
// whose first segment is a project name (not a known Anvil type) is reported
// as unresolved. The link-indexer silently drops such wikilinks, so accepting
// them at create time would produce a silent graph orphan.
func TestResolveBodyLinks_BareProjectSlugFlagged(t *testing.T) {
	v := newScaffolded(t)
	body := "See [[anvil.consolidate-anvil-surface]] for context.\n"
	got := ResolveBodyLinks(v, body)
	want := []UnresolvedLink{{Field: "body", Target: "anvil.consolidate-anvil-surface"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveBodyLinks_WhitespacePaddedBareProjectSlugFlagged asserts that a
// bare project.slug body wikilink with surrounding whitespace is still flagged.
// The indexer trims the token before lookup, so an un-trimmed validator would
// accept `[[ anvil.foo ]]` while the indexer produces zero edges — a silent
// graph orphan. Both paths must normalize identically.
func TestResolveBodyLinks_WhitespacePaddedBareProjectSlugFlagged(t *testing.T) {
	v := newScaffolded(t)
	body := "See [[ anvil.consolidate-anvil-surface ]] for context.\n"
	got := ResolveBodyLinks(v, body)
	want := []UnresolvedLink{{Field: "body", Target: "anvil.consolidate-anvil-surface"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveBodyLinks_NoSpaceAliasNormalized asserts that an aliased body
// wikilink without surrounding spaces (`[[type.project.slug|Alias]]`) is
// normalized the same as the indexer: the alias is stripped before type/target
// lookup. A dangling target is flagged; a resolving target is accepted. Before
// the fix, create stat'd the literal `…|Alias.md` path and mis-rejected.
func TestResolveBodyLinks_NoSpaceAliasNormalized(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.real")
	cases := []struct {
		name string
		body string
		want []UnresolvedLink
	}{
		{
			name: "dangling aliased link flagged on normalized target",
			body: "See [[issue.anvil.ghost|Display]] for context.\n",
			want: []UnresolvedLink{{Field: "body", Target: "issue.anvil.ghost"}},
		},
		{
			name: "resolving aliased link accepted",
			body: "See [[issue.anvil.real|the real issue]] for context.\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBodyLinks(v, tc.body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestResolveBodyLinks_TwoFencedBlocksProseInBetween exercises the non-greedy
// [\s\S]*? in fencedBlockRe: the first closing fence must not consume the
// second fenced block, so only the prose wikilink between them is validated.
func TestResolveBodyLinks_TwoFencedBlocksProseInBetween(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.real")
	body := "```bash\necho [[issue.anvil.ghost1]]\n```\n\nSee [[issue.anvil.real]] for context.\n\n```go\nfmt.Println([[issue.anvil.ghost2]])\n```\n"
	got := ResolveBodyLinks(v, body)
	// Only the prose link is scanned; ghost1 and ghost2 are inside fenced blocks.
	// anvil.real resolves, so no unresolved links expected.
	if len(got) != 0 {
		t.Errorf("expected 0 unresolved, got %v", got)
	}
}

// TestWikilinkTargetExists pins the predicate/reporter split: ResolveLinks
// returns nothing both for "resolves" and for "not a link at all", so a write
// path must not read its empty result as existence. Every non-artifact form —
// placeholder, whitespace, no dot, unknown type — is false here.
func TestWikilinkTargetExists(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "anvil.real")
	cp := filepath.Join(v.Root, TypeConvention.Dir(), "convention.sqlmesh.md")
	if err := os.MkdirAll(filepath.Dir(cp), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(cp, []byte("---\ntype: convention\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cases := []struct {
		target string
		want   bool
	}{
		{"issue.anvil.real", true},
		{"convention.sqlmesh", true},
		{"issue.anvil.ghost", false},
		{"system-design.<project>", false},
		{"convention.sqlmesh bad", false},
		{"convention.sqlmesh\tbad", false},
		{"nodot", false},
		{"notatype.thing", false},
	}
	for _, tc := range cases {
		if got := WikilinkTargetExists(v, tc.target); got != tc.want {
			t.Errorf("WikilinkTargetExists(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}

// TestArtifactBasename pins the both-shapes rule: a file whose name carries its
// type prefix must read back under the same id as the bare-named shape, while
// design and convention ids — which have always kept their prefix on disk —
// keep resolving exactly as before.
func TestArtifactBasename(t *testing.T) {
	v := newScaffolded(t)
	writeBlankIssue(t, v, "issue.demo.0001.probe")
	writeBlankIssue(t, v, "demo.0002.plain")
	cp := filepath.Join(v.Root, TypeConvention.Dir(), "convention.sqlmesh.md")
	if err := os.MkdirAll(filepath.Dir(cp), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(cp, []byte("---\ntype: convention\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	// A design doc minted before its prefix was dropped: qualified filename on
	// disk, bare canonical id.
	dp := filepath.Join(v.Root, TypeProductDesign.Dir(), "product-design.burgh.md")
	if err := os.MkdirAll(filepath.Dir(dp), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(dp, []byte("---\ntype: product-design\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cases := []struct {
		t    Type
		raw  string
		want string
	}{
		{TypeIssue, "demo.0001.probe", "issue.demo.0001.probe"},
		{TypeIssue, "issue.demo.0001.probe", "issue.demo.0001.probe"},
		{TypeIssue, "demo.0002.plain", "demo.0002.plain"},
		{TypeIssue, "issue.demo.0002.plain", "demo.0002.plain"},
		{TypeIssue, "demo.0003.ghost", "issue.demo.0003.ghost"},
		// A doubled prefix must not fall back onto the plain file.
		{TypeIssue, "issue.issue.demo.0002.plain", "issue.issue.demo.0002.plain"},
		{TypeConvention, "sqlmesh", "convention.sqlmesh"},
		{TypeConvention, "convention.sqlmesh", "convention.sqlmesh"},
		{TypeConvention, "convention.convention.sqlmesh", "convention.convention.sqlmesh"},
		// Qualified back-catalogue fallback: bare canonical id, old
		// type-qualified file still on disk.
		{TypeProductDesign, "burgh", "product-design.burgh"},
		{TypeProductDesign, "product-design.burgh", "product-design.burgh"},
		{TypeProductDesign, "ghost", "ghost"},
	}
	for _, tc := range cases {
		if got := ArtifactBasename(v, tc.t, tc.raw); got != tc.want {
			t.Errorf("ArtifactBasename(%s, %q) = %q, want %q", tc.t, tc.raw, got, tc.want)
		}
	}
}
