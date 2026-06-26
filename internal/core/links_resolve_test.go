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
// flat layout. Design ids keep the type prefix for global uniqueness, so the
// on-disk id is the full wikilink target (e.g. system-design.burgh).
func TestResolveLinks_DesignDocPresent(t *testing.T) {
	v := newScaffolded(t)
	files := map[Type][]string{
		TypeProductDesign: {"product-design.burgh"},
		TypeSystemDesign:  {"system-design.burgh", "system-design.anvil.build"},
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
