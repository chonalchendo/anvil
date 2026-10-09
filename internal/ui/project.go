package ui

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// lately caps the prose bands: the newest decisions, learnings and open threads.
const lately = 5

type invSeg struct {
	Type, Status string
	N            int
}

type invStatus struct {
	Type, Status, Glyph string
	N                   int
}

type invPart struct{ Text, Href string }

// invRow is one inventory line: a type's count, stacked status bar and per-status counts.
// Parts is the per-type tally of a merged row.
type invRow struct {
	Label, Href string
	Count       int
	Segs        []invSeg
	Statuses    []invStatus
	Parts       []invPart
}

// proseItem is a node with the short date it was last updated, for the prose bands.
// Sep is the text that joins it to the item before it in a sentence.
type proseItem struct {
	node
	Updated, Sep string
}

type projectPage struct {
	Name, Deck    string
	Inventory     []invRow
	Flight        []flight
	Done          []doneRow
	Designs       designs
	Decided       []proseGroup
	Learned       []proseGroup
	LearnedDrafts int
	OpenThreads   []proseItem
}

// project serves one project's dashboard; a project with no artifacts is a 404.
func (s *server) project(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("slug")
	counts, err := s.db.CountByTypeStatus(name)
	if err != nil {
		slog.Error("counting project", "project", name, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	if len(counts) == 0 {
		http.NotFound(w, r)
		return
	}
	page, err := s.buildProject(name, counts)
	if err != nil {
		slog.Error("building project page", "project", name, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "project", page)
}

func (s *server) buildProject(name string, counts map[string]map[string]int) (projectPage, error) {
	page := projectPage{Name: name, LearnedDrafts: counts["learning"]["draft"]}
	threads, err := s.projectThreads(name)
	if err != nil {
		return page, err
	}
	// Threads carry no project, so the index count is replaced by the topic match.
	counts["thread"] = map[string]int{}
	for _, t := range threads {
		counts["thread"][t.Status]++
		if t.Status == "open" && len(page.OpenThreads) < lately {
			page.OpenThreads = append(page.OpenThreads, proseItem{node: leaf(t), Updated: shortDate(t.Updated)})
		}
	}
	joinProse(page.OpenThreads)
	page.Inventory = inventory(name, counts)
	for _, build := range []func(*projectPage, map[string]map[string]int) error{s.fillDesigns, s.fillFlight, s.fillLately} {
		if err := build(&page, counts); err != nil {
			return page, err
		}
	}
	return page, nil
}

// projectThreads returns the project's threads, newest first. A thread belongs to a project
// by topic: the slug itself, or a topic opening with the slug and a hyphen.
func (s *server) projectThreads(project string) ([]index.ArtifactRow, error) {
	all, err := s.db.ListByType("thread", index.QueryFilters{})
	if err != nil {
		return nil, err
	}
	var out []index.ArtifactRow
	for _, r := range all {
		topic, _, _, ok := core.SplitTopicOrdinal(strings.TrimPrefix(r.ID, "thread."))
		if ok && (topic == project || strings.HasPrefix(topic, project+"-")) {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, byNewest)
	return out, nil
}

// inventory builds one row per type the project holds, the three design types merged into one;
// conventions are shared and sessions are not state.
func inventory(project string, counts map[string]map[string]int) []invRow {
	var rows []invRow
	for _, g := range sidebarLayout {
		merged := map[string]int{}
		var parts []invPart
		for _, t := range g.types {
			byStatus := counts[t.typ]
			if len(byStatus) == 0 || t.typ == "convention" || t.typ == "session" {
				continue
			}
			href := "/type/" + t.typ + "?project=" + url.QueryEscape(project)
			if t.typ == "thread" {
				href = "/type/thread"
			}
			if g.name != "Design" {
				rows = append(rows, invRowOf(t.typ, strings.ToLower(t.label), href, byStatus))
				continue
			}
			n := 0
			for st, c := range byStatus {
				merged[st] += c
				n += c
			}
			label, _ := strings.CutSuffix(strings.ToLower(t.label), " designs")
			parts = append(parts, invPart{Text: groupThousands(n) + " " + label, Href: href})
		}
		if len(parts) > 0 {
			row := invRowOf("design", "designs", parts[0].Href, merged)
			row.Parts = parts
			rows = append(rows, row)
		}
	}
	return rows
}

func invRowOf(typ, label, href string, byStatus map[string]int) invRow {
	row := invRow{Label: label, Href: href}
	statuses := make([]string, 0, len(byStatus))
	for st := range byStatus {
		statuses = append(statuses, st)
	}
	slices.SortFunc(statuses, func(a, b string) int {
		if d := rank(liveOrder, a) - rank(liveOrder, b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	for _, st := range statuses {
		n := byStatus[st]
		row.Count += n
		row.Segs = append(row.Segs, invSeg{Type: typ, Status: st, N: n})
		row.Statuses = append(row.Statuses, invStatus{Type: typ, Status: st, Glyph: glyphs[st], N: n})
	}
	return row
}

// shortDate turns "2026-10-09" or a timestamp into "9 Oct"; anything else passes through.
func shortDate(s string) string {
	if len(s) >= 10 {
		if t, err := time.Parse(time.DateOnly, s[:10]); err == nil {
			return t.Format("2 Jan")
		}
	}
	return s
}
