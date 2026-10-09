package ui

import (
	"strings"
	"testing"
)

const sectionBody = "intro\n\n## One\n\n- a\n- b\n\n```\n## not a heading\n```\n\n## Two\n\ntext\n\n### Sub\n"

func TestSection_WrapsEachH2(t *testing.T) {
	out, err := newMarkdown(resolver{}).renderSections(sectionBody)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if n := strings.Count(got, `<details class="section"`); n != 2 {
		t.Fatalf("sections = %d, want 2:\n%s", n, got)
	}
	for _, h := range []string{"One", "Two"} {
		if !strings.Contains(got, `<summary><h2>`+h+`</h2>`+"\n"+`</summary>`) {
			t.Errorf("section %s lacks a bare summary:\n%s", h, got)
		}
	}
	if !strings.HasPrefix(got, "<p>intro</p>") {
		t.Errorf("preamble must stay outside sections:\n%s", got)
	}
	if !strings.Contains(got, "## not a heading") || !strings.Contains(got, "<h3>Sub</h3>") {
		t.Errorf("fenced block or H3 changed:\n%s", got)
	}
}

func TestFold_StackStaysFlatAndArtifactFolds(t *testing.T) {
	h, _ := seed(t)
	_, stack := do(h, "GET", stackPath)
	if strings.Contains(stack, `<details class="section"`) {
		t.Error("stack must not fold sections")
	}
	if !strings.Contains(stack, `details class="layer"`) {
		t.Error("stack lost its layers")
	}
	_, page := do(h, "GET", "/artifact/product-design.anvil")
	if !strings.Contains(page, `<details class="section"`) {
		t.Error("artifact page has no folded section")
	}
}

func TestSection_H1ClosesOpenSection(t *testing.T) {
	out, err := newMarkdown(resolver{}).renderSections("## One\n\ntext\n\n# Top\n\nafter\n")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if n := strings.Count(got, `<details class="section"`); n != 1 {
		t.Fatalf("sections = %d, want 1:\n%s", n, got)
	}
	if strings.Index(got, "<h1>Top</h1>") < strings.LastIndex(got, "</details>") {
		t.Errorf("H1 sits inside the previous section:\n%s", got)
	}
}

// Warrant: an empty `##` has no title line; reading it unguarded panics the handler.
func TestSection_EmptyHeadingRendersWithoutPanic(t *testing.T) {
	md := newMarkdown(resolver{})
	if _, err := md.renderSections("##\n\nx\n"); err != nil {
		t.Fatal(err)
	}
	pb, err := md.renderPage("##\n\nx\n\n## Two\n\ny\n", resolver{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Outline) != 2 || pb.Outline[0].Title != "" || pb.Outline[0].Size != "1 line" || pb.Outline[1].Size != "1 line" {
		t.Errorf("outline = %+v", pb.Outline)
	}
}
