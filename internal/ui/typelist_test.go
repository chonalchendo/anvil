package ui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestTypeList_FiltersDiscriminate(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "other.0001-elsewhere", map[string]any{
		"title": "Elsewhere", "project": "other", "status": "superseded",
	}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0002-second", map[string]any{
		"title": "Second one", "project": "anvil", "status": "accepted",
	}, "x\n")
	const first, other, second = "/artifact/decision.ui.0001-a-decision", "/artifact/decision.other.0001-elsewhere", "/artifact/decision.ui.0002-second"
	cases := []struct {
		query         string
		want, exclude []string
	}{
		{"", []string{first, other, second}, nil},
		{"?project=other", []string{other}, []string{first, second}},
		{"?status=accepted", []string{second}, []string{first, other}},
		{"?project=anvil&status=accepted", []string{second}, []string{first, other}},
	}
	for _, c := range cases {
		code, body := do(h, "GET", "/type/decision"+c.query)
		if code != 200 {
			t.Fatalf("%q = %d", c.query, code)
		}
		for _, w := range c.want {
			if !strings.Contains(body, w) {
				t.Errorf("%q lacks %s", c.query, w)
			}
		}
		for _, x := range c.exclude {
			if strings.Contains(body, x) {
				t.Errorf("%q should not list %s", c.query, x)
			}
		}
	}
	if _, body := do(h, "GET", "/type/decision?status=accepted"); !strings.Contains(body, "Second one") {
		t.Error("row lacks its title")
	}
}

// Warrant: if the comparator ignored status or updated, groups would repeat or rows misorder.
func TestTypeList_GroupsLiveFirstNewestFirst(t *testing.T) {
	h, v := seed(t)
	for id, fm := range map[string][2]string{
		"ui.0010-older": {"accepted", "2026-01-01"},
		"ui.0011-newer": {"accepted", "2026-03-01"},
		"ui.0012-live":  {"proposed", "2026-02-01"},
		"ui.0013-dead":  {"rejected", "2026-04-01"},
		"ui.0014-gone":  {"stale", "2026-04-02"},
		"ui.0015-odd":   {"weird", "2026-04-03"},
	} {
		writeArtifact(t, v, core.TypeDecision, id, map[string]any{
			"title": id, "project": "anvil", "status": fm[0], "updated": fm[1],
		}, "x\n")
	}
	_, body := do(h, "GET", "/type/decision")
	at := func(id string) int {
		i := strings.Index(body, "/artifact/decision."+id)
		if i < 0 {
			t.Fatalf("%s not rendered", id)
		}
		return i
	}
	if at("ui.0011-newer") > at("ui.0010-older") {
		t.Error("older decision listed before newer in one group")
	}
	for _, c := range [][2]string{
		{"ui.0012-live", "ui.0010-older"},
		{"ui.0010-older", "ui.0014-gone"},
		{"ui.0014-gone", "ui.0013-dead"},
		{"ui.0013-dead", "ui.0015-odd"},
	} {
		if at(c[0]) > at(c[1]) {
			t.Errorf("%s should precede %s", c[0], c[1])
		}
	}
	for _, st := range []string{"accepted", "proposed", "rejected", "stale", "weird"} {
		if n := strings.Count(body, `aria-label="`+st+`"`); n != 1 {
			t.Errorf("status heading %q appears %d times, want 1", st, n)
		}
	}
	for _, st := range []string{"accepted", "proposed", "rejected", "stale"} {
		if want := glyphs[st] + " " + st + "</span>"; glyphs[st] == "" || !strings.Contains(body, want) {
			t.Errorf("page lacks status mark %q", want)
		}
	}
}

// Warrant: if a schema-legal status lacked a glyph, its heading would render bare.
func TestTypeList_GlyphsForOtherTypes(t *testing.T) {
	h, v := seed(t)
	cases := []struct {
		typ    core.Type
		status string
	}{
		{core.TypeInbox, "triaged"},
		{core.TypeSession, "distilled"},
		{core.TypeSession, "archived"},
		{core.TypeSweep, "merged"},
	}
	for i, c := range cases {
		writeArtifact(t, v, c.typ, fmt.Sprintf("ui.%04d-x", i), map[string]any{
			"title": "x", "project": "anvil", "status": c.status, "updated": "2026-01-01",
		}, "x\n")
	}
	for _, c := range cases {
		_, body := do(h, "GET", "/type/"+string(c.typ))
		if want := glyphs[c.status] + " " + c.status + "</span>"; glyphs[c.status] == "" || !strings.Contains(body, want) {
			t.Errorf("%s page lacks status mark %q", c.typ, want)
		}
	}
}

func TestTypeList_UnknownType404(t *testing.T) {
	h, _ := seed(t)
	if code, _ := do(h, "GET", "/type/nonsense"); code != 404 {
		t.Errorf("unknown type = %d, want 404", code)
	}
}

// Warrant: a to= filter that ignored the link target would list every decision.
func TestTypeList_ToKeepsOnlyCitingArtifacts(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0002-loner", map[string]any{"title": "Loner"}, "x\n")
	_, body := do(h, "GET", "/type/decision?to=product-design.anvil")
	if !strings.Contains(body, "/artifact/decision.ui.0001-a-decision") {
		t.Error("citing decision missing")
	}
	if strings.Contains(body, "decision.ui.0002-loner") {
		t.Error("non-citing decision listed")
	}
}

func seedTyped(t *testing.T) http.Handler {
	t.Helper()
	h, _ := seedTypedVault(t)
	return h
}

func seedTypedVault(t *testing.T) (http.Handler, *core.Vault) {
	t.Helper()
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0002-second", map[string]any{
		"title": "Second one", "project": "anvil", "status": "accepted", "tags": []any{"domain/ui", "type/decision"},
		"related": []any{"[[product-design.anvil]]"},
	}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0003-third", map[string]any{
		"title": "Third one", "project": "other", "status": "accepted", "tags": []any{"domain/cli"},
	}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0004-fourth", map[string]any{
		"title": "Fourth one", "project": "anvil", "status": "proposed",
	}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0900-cites", map[string]any{
		"title": "Cites", "project": "anvil", "related": []any{"[[decision.ui.0002-second]]"},
	}, "x\n")
	return h, v
}

// Warrant: tab counts taken before the project filter, or hrefs dropping a filter, would mislead or break composition.
func TestTypeListTabs(t *testing.T) {
	h := seedTyped(t)
	_, body := do(h, "GET", "/type/decision?project=anvil")
	_, tabs, _ := strings.Cut(body, `<nav class="tabs"`)
	tabs, _, _ = strings.Cut(tabs, "</nav>")
	for _, want := range []string{
		`>All <span class="count">2</span>`,
		`href="/type/decision?project=anvil&amp;status=accepted"`, `>accepted <span class="count">1</span>`,
	} {
		if !strings.Contains(tabs, want) {
			t.Errorf("tabs lack %q in\n%s", want, tabs)
		}
	}
	if !strings.Contains(tabs, `href="/type/decision?project=anvil" aria-current="page">All`) {
		t.Errorf("All tab is not current or lost the project filter:\n%s", tabs)
	}
	if strings.Contains(tabs, "other") {
		t.Error("tabs leak the other project")
	}
	h, v := seedTypedVault(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0005-no-tag", map[string]any{
		"title": "No tag", "project": "anvil", "status": "accepted", "tags": []any{"domain/ui"},
		"related": []any{"[[product-design.anvil]]"},
	}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0006-no-link", map[string]any{
		"title": "No link", "project": "anvil", "status": "accepted", "tags": []any{"type/decision"},
	}, "x\n")
	_, body = do(h, "GET", "/type/decision?project=anvil&status=accepted&tag=type/decision&to=product-design.anvil")
	if !strings.Contains(body, "decision.ui.0002-second") {
		t.Error("matching decision missing under the combined filter")
	}
	for _, gone := range []string{"decision.ui.0001-a-decision", "decision.ui.0005-no-tag", "decision.ui.0006-no-link", "decision.ui.0003-third", "decision.ui.0004-fourth"} {
		if strings.Contains(body, gone) {
			t.Errorf("%s survived the combined filter", gone)
		}
	}
	_, chipsHTML, _ := strings.Cut(body, `<div class="chips">`)
	chipsHTML, _, _ = strings.Cut(chipsHTML, "</div>")
	for _, want := range []string{
		`href="/type/decision?project=anvil&amp;status=accepted&amp;to=product-design.anvil" title="Remove filter">tag type/decision ×`,
		`href="/type/decision?project=anvil&amp;status=accepted&amp;tag=type%2Fdecision" title="Remove filter">cites product-design.anvil ×`,
		`href="/type/decision?status=accepted&amp;tag=type%2Fdecision&amp;to=product-design.anvil">all</a>`,
		`href="/type/decision?project=anvil&amp;status=accepted&amp;tag=type%2Fdecision&amp;to=product-design.anvil" aria-current="true">anvil</a>`,
		`href="/type/decision?project=other&amp;status=accepted&amp;tag=type%2Fdecision&amp;to=product-design.anvil">other</a>`,
	} {
		if !strings.Contains(chipsHTML, want) {
			t.Errorf("chips lack %q in\n%s", want, chipsHTML)
		}
	}
	if !strings.Contains(body, `aria-current="page">accepted`) {
		t.Error("accepted tab is not current")
	}
	_, body = do(h, "GET", "/type/decision?status=proposed&tag=domain/cli")
	if !strings.Contains(body, `aria-current="page">proposed <span class="count">0</span>`) {
		t.Errorf("empty active status lost its current tab:\n%s", body)
	}
	if _, body = do(h, "GET", "/type/decision?tag=domain/cli"); strings.Contains(body, "ui.0002-second") || !strings.Contains(body, "ui.0003-third") {
		t.Error("tag filter did not discriminate")
	}
}

// Warrant: a column bound to the wrong source would show a wrong id, tag or backlink count.
func TestTypeListColumns(t *testing.T) {
	h := seedTyped(t)
	_, body := do(h, "GET", "/type/decision")
	at := strings.Index(body, `decision.ui.0002-second">`)
	row, _, _ := strings.Cut(body[strings.LastIndex(body[:at], "<tr>"):], "</tr>")
	for _, want := range []string{
		`<td class="status">`, `<td class="id"><a href="/artifact/decision.ui.0002-second">decision.ui.0002-second</a></td>`,
		`<td class="title">Second one</td>`, `<td class="tags">`, `>domain/ui</a>`, `href="/type/decision?tag=domain%2Fui"`,
		`<td class="backlinks">1</td>`, `<td class="updated">`,
	} {
		if !strings.Contains(row, want) {
			t.Errorf("row lacks %q in\n%s", want, row)
		}
	}
	if !strings.Contains(body, `<footer class="keys">`) {
		t.Error("type list lost the key hint bar")
	}
}

// Warrant: a type missing from the map, or sharing a glyph, would render a blank or ambiguous icon.
func TestTypeIcons(t *testing.T) {
	seen := map[string]core.Type{}
	for _, typ := range core.AllTypes {
		ic := typeIcons[string(typ)]
		if ic == "" {
			t.Errorf("%s has no icon", typ)
		}
		if other, dup := seen[ic]; dup {
			t.Errorf("%s and %s share icon %q", typ, other, ic)
		}
		seen[ic] = typ
	}
	h, _ := seed(t)
	_, page := do(h, "GET", decisionPath)
	_, list := do(h, "GET", "/type/decision")
	icon := `>` + typeIcons["decision"] + `</span>`
	header, _, _ := strings.Cut(strings.SplitN(page, `<header class="node">`, 2)[1], "</header>")
	title, _, _ := strings.Cut(strings.SplitN(list, `<h1 class="node-title">`, 2)[1], "</h1>")
	for name, region := range map[string]string{"header": header, "type list": title, "sidebar": sidebarOf(t, list)} {
		if !strings.Contains(region, icon) {
			t.Errorf("%s lacks the decision icon", name)
		}
	}
}

// Warrant: fails if ?topic= stops filtering decisions and threads by id prefix, or drops the chip.
func TestTypeList_TopicFilter(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/type/decision?topic=gamma")
	if !strings.Contains(body, "decision.gamma.0001-a") || strings.Contains(body, "decision.alpha") || !strings.Contains(body, "topic gamma") {
		t.Errorf("topic filter wrong:\n%s", body)
	}
}
