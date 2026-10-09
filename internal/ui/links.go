package ui

import (
	"cmp"
	"html"
	"html/template"
	"net/url"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
	"github.com/yuin/goldmark/ast"
)

// link is one typed wikilink after resolution. Href is empty when unresolved.
// Plain marks ordinary text that is not a link at all. Type, Title and Hue
// come from the index for a resolved link; Hue names the status colour class
// the link's underline takes.
type link struct {
	Text  string
	Href  string
	Plain bool
	Type  string
	Title string
	Hue   string
}

// resolver decides every typed link by artifact existence, never by file name,
// so prefix-less types (product-design, decision, learning, thread) resolve.
type resolver struct {
	v  *core.Vault
	db *index.DB
}

// resolve maps a wikilink target (`type.id`, optional `|alias` and `#anchor`)
// to a link. A target whose type is unknown or whose file is absent keeps its
// raw text and an empty Href.
func (r resolver) resolve(target string) link {
	target = strings.TrimSpace(target)
	name, label, hasLabel := strings.Cut(target, "|")
	if !hasLabel {
		label = name
	}
	name, _, _ = strings.Cut(name, "#")
	l := link{Text: label}
	prefix, id, ok := strings.Cut(name, ".")
	if !ok || id == "" {
		return l
	}
	t, err := core.ParseType(prefix)
	if err != nil || !core.WikilinkTargetExists(r.v, name) {
		return l
	}
	key := core.IndexKey(t, id)
	l.Href = artifactHref(key)
	l.Type = string(t)
	// A target the index lacks keeps its link; only the status hue and title go.
	if row, err := r.db.GetArtifact(key); err == nil {
		l.Title = row.Title
		l.Hue = strings.TrimPrefix(hue(l.Type, row.Status), " status-")
		if l.Hue == "" {
			l.Hue = row.Status
		}
	}
	return l
}

func artifactHref(key string) string {
	return "/artifact/" + url.PathEscape(key)
}

func stackHref(key string) string {
	return "/issue/" + url.PathEscape(key) + "/stack"
}

// tabs is the issue-view switcher. A zero Stack means the page is not an issue.
type tabs struct {
	Issue, Stack, Current string
}

func issueTabs(key, current string) tabs {
	return tabs{Issue: artifactHref(key), Stack: stackHref(key), Current: current}
}

// linksSentence renders the distinct wikilinks under n as one sentence of
// typed titles: "decision <a>…</a>, issue <a>…</a> and …". It is empty when n holds no wikilink.
func linksSentence(res resolver, n ast.Node) template.HTML {
	var parts []string
	seen := map[string]bool{}
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		w, ok := c.(*wikilinkNode)
		if !entering || !ok || seen[w.target] {
			return ast.WalkContinue, nil
		}
		seen[w.target] = true
		l := res.resolve(w.target)
		if l.Href == "" {
			parts = append(parts, `<span class="unresolved">`+html.EscapeString(l.Text)+`</span>`)
			return ast.WalkContinue, nil
		}
		title := cmp.Or(l.Title, l.Text)
		parts = append(parts, strings.ReplaceAll(l.Type, "-", " ")+` <a href="`+html.EscapeString(l.Href)+`"`+hueAttr(l)+`>`+html.EscapeString(title)+`</a>`)
		return ast.WalkContinue, nil
	})
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return template.HTML(parts[0] + ".") //nolint:gosec // parts are escaped above
	}
	return template.HTML(strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1] + ".") //nolint:gosec // parts are escaped above
}
