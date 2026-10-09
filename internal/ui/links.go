package ui

import (
	"net/url"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// link is one typed wikilink after resolution. Href is empty when unresolved.
// Plain marks ordinary text that is not a link at all.
type link struct {
	Text  string
	Href  string
	Plain bool
}

// resolver decides every typed link by artifact existence, never by file name,
// so prefix-less types (product-design, decision, learning, thread) resolve.
type resolver struct {
	v *core.Vault
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
	l.Href = artifactHref(core.IndexKey(t, id))
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
