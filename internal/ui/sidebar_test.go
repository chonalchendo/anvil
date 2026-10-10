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
		`href="/type/decision"><span class="ticon type-decision" aria-hidden="true">` + typeIcons["decision"] + `</span>Decisions<span class="count">2</span>`,
		`href="/type/thread"><span class="ticon type-thread" aria-hidden="true">` + typeIcons["thread"] + `</span>Threads<span class="count">1</span>`,
		`href="/type/session"><span class="ticon type-session" aria-hidden="true">` + typeIcons["session"] + `</span>Sessions<span class="count">0</span>`,
		`<ul class="projects">`, `href="/project/a%20b&amp;c"`, `<details class="browse">`, `read-only`,
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

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 12345: "12,345", 1234567: "1,234,567"} {
		if got := groupThousands(n); got != want {
			t.Errorf("groupThousands(%d) = %q, want %q", n, got, want)
		}
	}
}

// Warrant: fails if a project loses its dashboard link, the current one is unmarked, or Browse opens by default.
func TestSidebar_ProjectsFirstAndBrowseClosed(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/project/anvil")
	sb := sidebarOf(t, body)
	if !strings.Contains(sb, `<a href="/project/anvil" aria-current="page">anvil</a>`) {
		t.Errorf("current project is not marked:\n%s", sb)
	}
	if strings.Index(sb, `class="projects"`) > strings.Index(sb, `class="browse"`) {
		t.Error("Projects must come before Browse")
	}
	if strings.Contains(sb, `<details class="browse" open`) {
		t.Error("Browse opens by default")
	}
}

// Warrant: fails if the sidebar loses the Knowledge link, marks it off the knowledge and topic pages, or marks it elsewhere.
func TestSidebar_KnowledgeLink(t *testing.T) {
	h := topicVault(t)
	for path, current := range map[string]string{"/": "page", "/topic/alpha": "true", "/type/decision": ""} {
		_, body := do(h, "GET", path)
		sb := sidebarOf(t, body)
		want := `<a href="/"` + map[bool]string{true: ` aria-current="` + current + `"`, false: ""}[current != ""] + `>Knowledge</a>`
		if !strings.Contains(sb, want) {
			t.Errorf("%s: sidebar lacks %s", path, want)
		}
		if strings.Index(sb, "Knowledge</a>") > strings.Index(sb, `aria-label="Projects"`) {
			t.Errorf("%s: Knowledge must come before Projects", path)
		}
	}
}
