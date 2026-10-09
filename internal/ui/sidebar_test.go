package ui

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func sidebarOf(t *testing.T, body string) string {
	t.Helper()
	a := strings.Index(body, `<aside class="sidebar">`)
	b := strings.Index(body, `</aside>`)
	if a < 0 || b < a {
		t.Fatal("page has no sidebar")
	}
	return body[a:b]
}

// Warrant: a count rendered from the wrong query or a missing group would leave a type unreachable or mislabelled.
func TestSidebar_CarriesGroupsCountsAndProjects(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0002-second", map[string]any{
		"title": "Second", "project": "a b&c",
	}, "x\n")
	_, body := do(h, "GET", "/artifact/product-design.anvil")
	sb := sidebarOf(t, body)
	for _, want := range []string{
		`id="palette-q"`, `>Design</div>`, `>Work</div>`, `>Knowledge</div>`, `>Capture</div>`,
		`href="/type/decision">decision<span class="count">2</span>`,
		`href="/type/thread">thread<span class="count">1</span>`,
		`href="/type/session">session<span class="count">0</span>`,
		`<ul class="projects">`, `href="/type/issue?project=a&#43;b%26c"`, `read-only`,
	} {
		if !strings.Contains(sb, want) {
			t.Errorf("sidebar lacks %q in\n%s", want, sb)
		}
	}
	if n := strings.Count(sb, `class="count"`); n != 11 {
		t.Errorf("count spans = %d, want 11", n)
	}
}

func TestSidebar_RendersOnEveryPage(t *testing.T) {
	h, _ := seed(t)
	for _, p := range []string{"/", "/type/decision", decisionPath, stackPath} {
		_, body := do(h, "GET", p)
		if !strings.Contains(sidebarOf(t, body), `class="group"`) {
			t.Errorf("%s lacks the grouped sidebar", p)
		}
	}
}
