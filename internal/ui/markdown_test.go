package ui

import (
	"strings"
	"testing"
)

const sectionBody = "intro\n\n## One\n\n- a\n- b\n\n```\n## not a heading\n```\n\n## Two\n\ntext\n\n### Sub\n"

func TestSection_WrapsEachH2WithCount(t *testing.T) {
	out, err := newMarkdown(resolver{}).renderSections(sectionBody)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if n := strings.Count(got, `<details class="section" open>`); n != 2 {
		t.Fatalf("sections = %d, want 2:\n%s", n, got)
	}
	if !strings.Contains(got, `<summary><h2>One</h2>`+"\n"+`<span class="count">2</span></summary>`) {
		t.Errorf("section One lacks summary with count 2:\n%s", got)
	}
	if !strings.Contains(got, `<summary><h2>Two</h2>`+"\n"+`</summary>`) {
		t.Errorf("section Two should have no count:\n%s", got)
	}
	if !strings.HasPrefix(got, "<p>intro</p>") {
		t.Errorf("preamble must stay outside sections:\n%s", got)
	}
	if !strings.Contains(got, "## not a heading") || !strings.Contains(got, "<h3>Sub</h3>") {
		t.Errorf("fenced block or H3 changed:\n%s", got)
	}
}

func TestFold_StackRenderStaysFlatAndKeysCoverSections(t *testing.T) {
	out, err := newMarkdown(resolver{}).render(sectionBody)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "<details") {
		t.Error("render must not fold; stack layers share it")
	}
	h, _ := seed(t)
	_, page := do(h, "GET", "/artifact/product-design.anvil")
	if !strings.Contains(page, `<details class="section"`) {
		t.Error("artifact page has no folded section")
	}
	if !strings.Contains(page, "details.layer, details.section") {
		t.Error("key handler does not cover details.section")
	}
}
