package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestNodeHeader_ShowsIdentityAndSlots(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/"+stackIssue)
	i := strings.Index(body, `<header class="node">`)
	j := strings.Index(body, `</header>`)
	if i < 0 || j < i {
		t.Fatal("node header missing")
	}
	head := body[i:j]
	for _, want := range []string{"<h1>Thing</h1>", `class="status status-in-progress"`, "●", "in-progress", `<p class="deck">Deck line</p>`, `<p class="state">`, "updated 2026-10-09"} {
		if !strings.Contains(head, want) {
			t.Errorf("header lacks %q", want)
		}
	}
	for _, no := range []string{`class="key"`, "milestone.anvil.m1"} {
		if strings.Contains(head, no) {
			t.Errorf("header repeats %q that the contents column holds", no)
		}
	}
	contents := contentsOf(t, body)
	for _, want := range []string{`<code>` + stackIssue + `</code>`, `href="/artifact/milestone.anvil.m1"`} {
		if !strings.Contains(contents, want) {
			t.Errorf("contents lacks %q", want)
		}
	}
}

func contentsOf(t *testing.T, body string) string {
	t.Helper()
	_, rest, ok := strings.Cut(body, `<section class="contents"`)
	contents, _, ok2 := strings.Cut(rest, "</section>")
	if !ok || !ok2 {
		t.Fatal("contents column missing")
	}
	return contents
}

func TestProps_FoldedClosedAndHeaderKeysExcluded(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/"+stackIssue)
	bodyAt, propsAt := strings.Index(body, `<section class="body">`), strings.Index(body, `<details class="props">`)
	if bodyAt < 0 || propsAt < bodyAt {
		t.Error("section.body must render before details.props")
	}
	_, props, ok := strings.Cut(body, `<details class="props">`)
	props, _, ok2 := strings.Cut(props, "</details>")
	if !ok || !ok2 {
		t.Fatal("props block missing")
	}
	if strings.Contains(body, `<details class="props" open`) {
		t.Fatal("props must fold closed")
	}
	if !strings.Contains(props, "<dt>learnings</dt>") {
		t.Error("learnings missing from props")
	}
	for _, no := range []string{"<dt>title</dt>", "<dt>status</dt>"} {
		if strings.Contains(props, no) {
			t.Errorf("props repeats header field %s", no)
		}
	}
}

// Warrant: a fold that counted rows instead of sources, or skipped the cap, would misreport or flood.
func TestCited_GroupsCountsCapsAndOmitsEmpty(t *testing.T) {
	h, v := seed(t)
	for i := 2; i <= 10; i++ {
		fm := map[string]any{"title": "More", "related": []any{"[[product-design.anvil]]"}}
		if i == 10 {
			fm["updated"] = "2099-01-01"
		}
		writeArtifact(t, v, core.TypeDecision, fmt.Sprintf("ui.%04d-more", i), fm, "x\n")
	}
	_, body := do(h, "GET", "/artifact/product-design.anvil")
	_, cited, ok := strings.Cut(contentsOf(t, body), `<details class="cited">`)
	cited, _, ok2 := strings.Cut(cited, "</details>")
	if !ok || !ok2 {
		t.Fatal("cited-by fold missing")
	}
	for _, want := range []string{`<summary>11 artifacts</summary>`, `<h3>decision <span class="count">10</span>`, `<h3>milestone <span class="count">1</span>`, `<li class="more"><a href="/type/decision?to=product-design.anvil">2 more</a>`} {
		if !strings.Contains(cited, want) {
			t.Errorf("cited-by lacks %q", want)
		}
	}
	if n := strings.Count(cited, `href="/artifact/decision.`); n != 8 {
		t.Errorf("decision links = %d, want 8", n)
	}
	if !strings.Contains(cited, `href="/artifact/decision.ui.0010-more"`) {
		t.Error("newest source is cut by the cap")
	}
	for _, cut := range []string{"decision.ui.0008-more", "decision.ui.0009-more"} {
		if strings.Contains(cited, `href="/artifact/`+cut+`"`) {
			t.Errorf("older source %s survived the cap", cut)
		}
	}
	if strings.Contains(cited, "<h3>issue") || strings.Contains(cited, "<h3>learning") {
		t.Error("empty group rendered")
	}
}

// Warrant: a page with no incoming links must not render an empty fold.
func TestCited_NoIncomingRendersPlaceholderOnly(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/"+stackIssue)
	if strings.Contains(body, `<details class="cited"`) || !strings.Contains(body, "Nothing links here.") {
		t.Error("contents should render only the placeholder")
	}
}

// Warrant: the page must not list a target in a rail, a "Links out" block and
// the body at once; the contents column holds the only cited-by fold, closed.
func TestArtifactPage_NoRailNoLinksOutAndNothingListedTwice(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.cites", map[string]any{
		"title": "Cites", "product_design": "[[product-design.anvil]]",
	}, "## Why\n\nSee [[decision.ui.0001-a-decision]].\n\n## Links\n\n- [[decision.ui.0001-a-decision]] - the pick.\n- [[thread.anvil-design-docs.0002-x]] - the thread.\n")
	_, body := do(h, "GET", "/artifact/milestone.milestone.anvil.cites")
	for _, no := range []string{`class="rail"`, "Links out", `<h2>Links</h2>\n<ul>`} {
		if strings.Contains(body, no) {
			t.Errorf("page still holds %q", no)
		}
	}
	if !strings.Contains(body, `class="contents"`) {
		t.Fatal("contents column missing")
	}
	outside := regexp.MustCompile(`(?s)<details class="(?:props|cited)".*?</details>`).ReplaceAllString(body, "")
	if n := strings.Count(outside, `href="/artifact/decision.ui.0001-a-decision"`); n != 2 {
		t.Errorf("decision links outside the properties and cited-by folds = %d, want 2 (body, Links sentence)", n)
	}
	contents := contentsOf(t, body)
	for _, want := range []string{`<h2>On this page</h2>`, `href="#why"`, `<span class="c">1 line</span>`, `<p class="prose">decision <a href="/artifact/decision.ui.0001-a-decision">A decision</a> and thread <a href="/artifact/thread.anvil-design-docs.0002-x">A thread</a>.</p>`} {
		if !strings.Contains(contents, want) {
			t.Errorf("contents lacks %q", want)
		}
	}
	if strings.Contains(body, `id="links"`) {
		t.Error("the Links section also stayed in the body")
	}
}

// Warrant: a Links section with no wikilink would vanish if lifted; it must stay in the body.
func TestArtifactPage_LinksSectionWithoutWikilinksStaysInBody(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeThread, "plain-links", map[string]any{"title": "P"}, "## Links\n\nNone yet.\n")
	_, body := do(h, "GET", "/artifact/thread.plain-links")
	if !strings.Contains(body, "None yet.") || strings.Contains(contentsOf(t, body), "<h2>Links</h2>") {
		t.Error("prose-only Links section was lifted out of the body")
	}
}

// Warrant: a link to a node with a status must carry that status's hue so the
// underline tells the reader where it leads.
func TestLinks_CarryTargetStatusHue(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeIssue, "ui.0002-open", map[string]any{"title": "Open one", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0003-cites", map[string]any{"title": "C"}, "See [[issue.ui.0002-open]] and [[product-design.anvil]].\n")
	_, body := do(h, "GET", "/artifact/decision.ui.0003-cites")
	for _, want := range []string{`<a href="/artifact/issue.ui.0002-open" class="to-planned">`, `<a href="/artifact/product-design.anvil">`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}
