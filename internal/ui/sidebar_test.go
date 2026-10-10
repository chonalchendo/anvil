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

// Warrant: fails if the sidebar regrows the type groups, counts or badge, or loses search or a project link.
func TestSidebar_HoldsSearchKnowledgeAndProjectsOnly(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0002-second", map[string]any{
		"title": "Second", "project": "a b&c",
	}, "x\n")
	for _, p := range []string{"/", "/type/decision", decisionPath, stackPath} {
		_, body := do(h, "GET", p)
		sb := sidebarOf(t, body)
		for _, want := range []string{`id="palette-q"`, `>Knowledge</a>`, `<ul class="projects">`, `href="/project/a%20b&amp;c"`} {
			if !strings.Contains(sb, want) {
				t.Errorf("%s: sidebar lacks %q in\n%s", p, want, sb)
			}
		}
		for _, dead := range []string{`href="/type/`, `class="count"`} {
			if strings.Contains(sb, dead) {
				t.Errorf("%s: sidebar still holds %q", p, dead)
			}
		}
	}
}

// Warrant: fails if a project loses its dashboard link or the current one is unmarked.
func TestSidebar_CurrentProjectMarked(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/project/anvil")
	if sb := sidebarOf(t, body); !strings.Contains(sb, `<a href="/project/anvil" aria-current="page">anvil</a>`) {
		t.Errorf("current project is not marked:\n%s", sb)
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
