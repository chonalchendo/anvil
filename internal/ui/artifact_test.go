package ui

import (
	"fmt"
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
	for _, want := range []string{"<h1>Thing</h1>", `class="status status-in-progress"`, "●", "in-progress", `<p class="deck">Deck line</p>`, "updated 2026-10-09", `<code class="key">` + stackIssue, `href="/artifact/milestone.anvil.m1"`} {
		if !strings.Contains(head, want) {
			t.Errorf("header lacks %q", want)
		}
	}
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
	for _, no := range []string{"<dt>title</dt>", "<dt>status</dt>", "<dt>milestone</dt>"} {
		if strings.Contains(props, no) {
			t.Errorf("props repeats header field %s", no)
		}
	}
}

// Warrant: a rail that counted rows instead of sources, or skipped the cap, would misreport or flood.
func TestRail_GroupsCountsCapsAndOmitsEmpty(t *testing.T) {
	h, v := seed(t)
	for i := 2; i <= 10; i++ {
		fm := map[string]any{"title": "More", "related": []any{"[[product-design.anvil]]"}}
		if i == 10 {
			fm["updated"] = "2099-01-01"
		}
		writeArtifact(t, v, core.TypeDecision, fmt.Sprintf("ui.%04d-more", i), fm, "x\n")
	}
	_, body := do(h, "GET", "/artifact/product-design.anvil")
	_, rail, ok := strings.Cut(body, `<aside class="rail">`)
	rail, _, ok2 := strings.Cut(rail, "</aside>")
	if !ok || !ok2 {
		t.Fatal("rail missing")
	}
	if strings.Contains(body, "Hanging off this node") {
		t.Error("flat block still renders")
	}
	for _, want := range []string{`<h3>decision <span class="count">10</span>`, `<h3>milestone <span class="count">1</span>`, `<li class="more"><a href="/type/decision?to=product-design.anvil">2 more</a>`} {
		if !strings.Contains(rail, want) {
			t.Errorf("rail lacks %q", want)
		}
	}
	if n := strings.Count(rail, `href="/artifact/decision.`); n != 8 {
		t.Errorf("decision links = %d, want 8", n)
	}
	if !strings.Contains(rail, `href="/artifact/decision.ui.0010-more"`) {
		t.Error("newest source is cut by the cap")
	}
	for _, cut := range []string{"decision.ui.0008-more", "decision.ui.0009-more"} {
		if strings.Contains(rail, `href="/artifact/`+cut+`"`) {
			t.Errorf("older source %s survived the cap", cut)
		}
	}
	if strings.Contains(rail, "<h3>issue") || strings.Contains(rail, "<h3>learning") {
		t.Error("empty group rendered")
	}
}

// Warrant: a rail that rendered empty group sections would show headings with no sources.
func TestRail_NoIncomingRendersPlaceholderOnly(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/"+stackIssue)
	if !strings.Contains(body, `<aside class="rail">`) || strings.Contains(body, `<section><h3>`) || !strings.Contains(body, "Nothing links here.") {
		t.Error("rail should render only the placeholder")
	}
}
