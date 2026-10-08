package ui

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/chonalchendo/anvil/internal/index"
)

// node is one row of the spine tree. A node with no Href is a group heading.
type node struct {
	Href, Title, Status string
	Kids                []node
	Open                bool
}

type projectTree struct {
	Name  string
	Nodes []node
}

// milestoneOrder lists the status groups shown, in order; other statuses follow by name.
var milestoneOrder = []string{"in-progress", "open"}

func (s *server) home(w http.ResponseWriter, _ *http.Request) {
	trees, err := s.spineTrees()
	if err != nil {
		slog.Error("building spine tree", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.pages.render(w, "home", trees)
}

// spineTrees reads only the index: rows for titles and statuses, link rows for
// the spine slots.
func (s *server) spineTrees() ([]projectTree, error) {
	byProject := map[string]map[string][]index.ArtifactRow{}
	for _, typ := range []string{"product-design", "system-design", "component-design", "milestone", "issue"} {
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
	names := make([]string, 0, len(byProject))
	for n := range byProject {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]projectTree, 0, len(names))
	for _, n := range names {
		nodes, err := s.projectNodes(byProject[n])
		if err != nil {
			return nil, err
		}
		if n == "" {
			n = "(no project)"
		}
		out = append(out, projectTree{Name: n, Nodes: nodes})
	}
	return out, nil
}

func (s *server) projectNodes(rows map[string][]index.ArtifactRow) ([]node, error) {
	var nodes, unlinked []node
	for _, r := range rows["product-design"] {
		nodes = append(nodes, leaf(r))
	}
	comps := map[string][]node{}
	for _, r := range rows["component-design"] {
		parent, err := s.slotTarget(r.ID, "system_design")
		if err != nil {
			return nil, err
		}
		if parent == "" {
			unlinked = append(unlinked, leaf(r))
			continue
		}
		comps[parent] = append(comps[parent], leaf(r))
	}
	for _, r := range rows["system-design"] {
		n := leaf(r)
		n.Kids, n.Open = comps[r.ID], true
		nodes = append(nodes, n)
	}
	issues := map[string]index.ArtifactRow{}
	for _, r := range rows["issue"] {
		issues[r.ID] = r
	}
	byStatus := map[string][]node{}
	for _, r := range rows["milestone"] {
		n := leaf(r)
		if r.Status == "in-progress" {
			kids, err := s.activeIssues(r.ID, issues)
			if err != nil {
				return nil, err
			}
			n.Kids, n.Open = kids, true
		}
		parent, err := s.slotTarget(r.ID, "product_design")
		if err != nil {
			return nil, err
		}
		if parent == "" {
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
	t := r.Title
	if t == "" {
		t = r.ID
	}
	return node{Href: artifactHref(r.ID), Title: t, Status: r.Status}
}

// slotTarget returns the target of key's outgoing link in slot, or "" when unset.
func (s *server) slotTarget(key, slot string) (string, error) {
	rows, err := s.db.LinksFrom(key)
	if err != nil {
		return "", fmt.Errorf("links from %s: %w", key, err)
	}
	for _, r := range rows {
		if r.Relation == slot {
			return r.Target, nil
		}
	}
	return "", nil
}

// activeIssues lists the in-progress issues whose milestone slot names ms.
func (s *server) activeIssues(ms string, issues map[string]index.ArtifactRow) ([]node, error) {
	in, err := s.db.LinksTo(ms)
	if err != nil {
		return nil, fmt.Errorf("links to %s: %w", ms, err)
	}
	var out []node
	for _, l := range in {
		if i, ok := issues[l.Source]; ok && l.Relation == "milestone" && i.Status == "in-progress" {
			out = append(out, leaf(i))
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Href < out[b].Href })
	return out, nil
}

func statusOrder(m map[string][]node) []string {
	var rest []string
	for st := range m {
		if st != milestoneOrder[0] && st != milestoneOrder[1] {
			rest = append(rest, st)
		}
	}
	sort.Strings(rest)
	var out []string
	for _, st := range append(append([]string{}, milestoneOrder...), rest...) {
		if _, ok := m[st]; ok {
			out = append(out, st)
		}
	}
	return out
}
