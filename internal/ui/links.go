package ui

import (
	"cmp"
	"net/url"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// link is one typed wikilink after resolution. Href is empty when unresolved.
// Plain marks ordinary text that is not a link at all. Aliased marks a Text
// the author chose with `|alias`. Type, Title, Status and Hue come from the
// index for a resolved link; Hue names the status colour class the link's
// underline takes.
type link struct {
	Text    string
	Href    string
	Plain   bool
	Aliased bool
	Type    string
	Title   string
	Status  string
	Hue     string
}

// label is the text a body link shows: the author's alias, else the target's
// title, else the raw target.
func (l link) label() string {
	if !l.Aliased && l.Title != "" {
		return l.Title
	}
	return l.Text
}

func (l link) TypeWord() string { return strings.ReplaceAll(l.Type, "-", " ") }

// resolver decides every typed link by artifact existence, never by file name,
// so prefix-less types (product-design, decision, learning, thread) resolve.
// rows is the request's id → index row catalog, so no link costs a query.
type resolver struct {
	v    *core.Vault
	rows map[string]index.ArtifactRow
}

// resolve maps a wikilink target (`type.id`, optional `|alias` and `#anchor`)
// to a link. A target whose type is unknown or whose file is absent keeps its
// raw text and an empty Href.
func (r resolver) resolve(target string) link {
	target = strings.TrimSpace(target)
	name, label, aliased := strings.Cut(target, "|")
	if !aliased {
		label = name
	}
	name, _, _ = strings.Cut(name, "#")
	l := link{Text: label, Aliased: aliased}
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
	if row, ok := r.rows[key]; ok {
		l.Title, l.Status = row.Title, row.Status
		l.Hue = cmp.Or(hue(l.Type, row.Status), row.Status)
	}
	return l
}

func artifactHref(key string) string {
	return "/artifact/" + url.PathEscape(key)
}

func projectHref(slug string) string { return "/project/" + url.PathEscape(slug) }

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
