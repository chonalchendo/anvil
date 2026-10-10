package ui

import (
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// node is one linked artifact row: its href, title and status mark.
type node struct {
	Href, Title, Type, Status, Glyph string
}

// hue returns the hue name for a status whose colour depends on the type, else
// empty: an open issue is queued work (planned), an open thread stays open.
func hue(typ, status string) string {
	if typ == string(core.TypeIssue) && status == "open" {
		return "planned"
	}
	return ""
}

// liveOrder ranks statuses live-first, for issue lists and type lists; unknown statuses sort last.
var liveOrder = []string{
	"in-progress", "active", "open", "escalated", "planned", "draft", "proposed", "raw", "triaged", "paused",
	"accepted", "verified", "promoted", "closed", "resolved", "done", "distilled", "archived", "merged",
	"superseded", "retired", "deprecated", "stale", "rejected", "dropped", "retracted", "abandoned",
}

// glyphs are the status marks shown before the status text.
var glyphs = map[string]string{
	"in-progress": "●", "active": "●",
	"open": "○", "planned": "○", "draft": "○",
	"done": "✓", "resolved": "✓",
	"abandoned": "×", "superseded": "×", "retired": "×", "deprecated": "×",
	"escalated": "▲", "stale": "▲",
	"accepted": "✓", "verified": "✓", "closed": "✓", "promoted": "✓",
	"proposed": "○", "raw": "○", "triaged": "○", "paused": "○",
	"distilled": "✓", "archived": "✓", "merged": "✓",
	"rejected": "×", "dropped": "×", "retracted": "×",
}

// typeIcons maps every artifact type to its distinct icon; the node header, nav sidebar and type list read it.
var typeIcons = map[string]string{
	"inbox": "✉", "issue": "◎", "milestone": "⚑", "decision": "⚖", "learning": "✦", "thread": "≋",
	"sweep": "⌁", "session": "◷", "product-design": "◈", "system-design": "▦", "component-design": "▣", "convention": "¶",
}

func leaf(r index.ArtifactRow) node {
	return node{Href: artifactHref(r.ID), Title: r.Title, Type: r.Type, Status: r.Status, Glyph: glyphs[r.Status]}
}

func rank(order []string, st string) int {
	for i, o := range order {
		if o == st {
			return i
		}
	}
	return len(order)
}

// byNewest orders rows by updated date, newest first.
func byNewest(a, b index.ArtifactRow) int { return strings.Compare(b.Updated, a.Updated) }
