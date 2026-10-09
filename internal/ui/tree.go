package ui

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// node is one row of the spine tree. A node with no Href is a group heading.
type node struct {
	Href, Title, Type, Status, Glyph string
	Kids                             []node
	Open                             bool
}

// hue returns the hue name for a status whose colour depends on the type, else
// empty: an open issue is queued work (planned), an open thread stays open.
func hue(typ, status string) string {
	if typ == string(core.TypeIssue) && status == "open" {
		return "planned"
	}
	return ""
}

// treeCap is how many children a group shows before folding the rest behind "N more".
const treeCap = 8

// Shown returns the children listed directly under the group.
func (n node) Shown() []node { return n.Kids[:min(len(n.Kids), treeCap)] }

// Hidden returns the children folded behind the "N more" row.
func (n node) Hidden() []node { return n.Kids[min(len(n.Kids), treeCap):] }

type projectTree struct {
	Name  string
	Nodes []node
}

// milestoneOrder lists the milestone status groups shown, in order.
var milestoneOrder = []string{"in-progress", "planned", "done", "abandoned"}

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

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	trees, err := s.spineTrees()
	if err != nil {
		slog.Error("building spine tree", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	band, err := s.nowBand()
	if err != nil {
		slog.Error("building now band", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "home", homePage{band, trees})
}

type homePage struct {
	Now   nowBand
	Trees []projectTree
}

// spineTrees reads only the index: rows for titles and statuses, link rows for
// the spine slots.
func (s *server) spineTrees() ([]projectTree, error) {
	byProject := map[string]map[string][]index.ArtifactRow{}
	for _, typ := range []string{"product-design", "system-design", "component-design", "milestone"} {
		rows, err := s.db.ListByType(typ, index.QueryFilters{})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if byProject[r.Project] == nil {
				byProject[r.Project] = map[string][]index.ArtifactRow{}
			}
			byProject[r.Project][typ] = append(byProject[r.Project][typ], r)
		}
	}
	issueRows, err := s.db.ListByType("issue", index.QueryFilters{})
	if err != nil {
		return nil, err
	}
	issues := map[string]index.ArtifactRow{}
	for _, r := range issueRows {
		issues[r.ID] = r
	}
	names := make([]string, 0, len(byProject))
	for n := range byProject {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]projectTree, 0, len(names))
	for _, n := range names {
		nodes, err := s.projectNodes(byProject[n], issues)
		if err != nil {
			return nil, err
		}
		out = append(out, projectTree{Name: n, Nodes: nodes})
	}
	return out, nil
}

func (s *server) projectNodes(rows map[string][]index.ArtifactRow, issues map[string]index.ArtifactRow) ([]node, error) {
	var nodes, unlinked []node
	for _, r := range rows["product-design"] {
		nodes = append(nodes, leaf(r))
	}
	comps := map[string][]node{}
	for _, r := range rows["component-design"] {
		out, err := s.db.LinksFrom(r.ID)
		if err != nil {
			return nil, fmt.Errorf("links from %s: %w", r.ID, err)
		}
		parent := slotOf(out, "system_design")
		if parent == "" {
			unlinked = append(unlinked, leaf(r))
			continue
		}
		comps[parent] = append(comps[parent], leaf(r))
	}
	for _, r := range rows["system-design"] {
		n := leaf(r)
		n.Kids, n.Open = comps[r.ID], true
		delete(comps, r.ID)
		nodes = append(nodes, n)
	}
	// A slot naming a missing or other-project system design is consumed by no row above.
	dangling := make([]string, 0, len(comps))
	for k := range comps {
		dangling = append(dangling, k)
	}
	sort.Strings(dangling)
	for _, k := range dangling {
		unlinked = append(unlinked, comps[k]...)
	}
	byStatus := map[string][]node{}
	for _, r := range rows["milestone"] {
		n := leaf(r)
		if r.Status == "in-progress" {
			kids, err := s.milestoneIssues(r.ID, issues)
			if err != nil {
				return nil, err
			}
			n.Kids, n.Open = kids, true
		}
		out, err := s.db.LinksFrom(r.ID)
		if err != nil {
			return nil, fmt.Errorf("links from %s: %w", r.ID, err)
		}
		if slotOf(out, "product_design") == "" {
			unlinked = append(unlinked, n)
			continue
		}
		byStatus[r.Status] = append(byStatus[r.Status], n)
	}
	for _, st := range statusOrder(byStatus) {
		nodes = append(nodes, node{Title: "Milestones · " + st, Kids: byStatus[st], Open: st == "in-progress"})
	}
	if len(unlinked) > 0 {
		nodes = append(nodes, node{Title: "Unlinked", Kids: unlinked, Open: true})
	}
	return nodes, nil
}

func leaf(r index.ArtifactRow) node {
	return node{Href: artifactHref(r.ID), Title: r.Title, Type: r.Type, Status: r.Status, Glyph: glyphs[r.Status]}
}

// milestoneIssues lists every issue whose milestone slot names ms, in status order.
func (s *server) milestoneIssues(ms string, issues map[string]index.ArtifactRow) ([]node, error) {
	in, err := s.db.LinksTo(ms)
	if err != nil {
		return nil, fmt.Errorf("links to %s: %w", ms, err)
	}
	var out []node
	for _, l := range in {
		if i, ok := issues[l.Source]; ok && l.Relation == "milestone" {
			out = append(out, leaf(i))
		}
	}
	sort.Slice(out, func(a, b int) bool {
		ra, rb := rank(liveOrder, out[a].Status), rank(liveOrder, out[b].Status)
		if ra != rb {
			return ra < rb
		}
		return out[a].Href < out[b].Href
	})
	return out, nil
}

func rank(order []string, st string) int {
	for i, o := range order {
		if o == st {
			return i
		}
	}
	return len(order)
}

func statusOrder(m map[string][]node) []string {
	var out []string
	for _, st := range milestoneOrder {
		if _, ok := m[st]; ok {
			out = append(out, st)
		}
	}
	return out
}
