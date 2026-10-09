package ui

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/index"
)

// lately caps the prose bands: the newest decisions, learnings and open threads.
const lately = 5

type invSeg struct {
	Class string
	N     int
}

type invStatus struct {
	Type, Status, Glyph string
	N                   int
}

// invRow is one inventory line: a type's count, stacked status bar and per-status counts.
type invRow struct {
	Label, Href string
	Count       int
	Segs        []invSeg
	Statuses    []invStatus
}

// proseItem is a node with the date it was last updated, for the prose bands.
type proseItem struct {
	node
	Updated string
}

type projectPage struct {
	Name, Deck    string
	Inventory     []invRow
	ConventionN   int
	Flight        []flight
	Done          []doneRow
	Designs       designs
	Decided       []proseItem
	Learned       []proseItem
	LearnedDrafts int
	OpenThreads   []proseItem
}

// barClasses maps a status to the stacked-bar hue; an unlisted status is retired.
var barClasses = map[string]string{
	"planned": "planned", "in-progress": "in-progress", "escalated": "in-progress", "open": "open",
	"done": "done", "resolved": "done", "accepted": "done", "active": "done", "verified": "done",
	"promoted": "done", "distilled": "done", "archived": "done", "merged": "done",
	"draft": "draft", "proposed": "draft", "raw": "draft", "triaged": "draft",
}

func barClass(typ, status string) string {
	if h := hue(typ, status); h != "" {
		return strings.TrimPrefix(h, " status-")
	}
	if c, ok := barClasses[status]; ok {
		return c
	}
	return "retired"
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
	page := projectPage{Name: name, Inventory: inventory(name, counts)}
	all, err := s.db.CountByType()
	if err != nil {
		return page, err
	}
	page.ConventionN = all["convention"]
	for _, build := range []func(*projectPage) error{s.fillDesigns, s.fillFlight, s.fillLately} {
		if err := build(&page); err != nil {
			return page, err
		}
	}
	return page, nil
}

// inventory builds one row per type the project holds; conventions are shared, sessions are not state.
func inventory(project string, counts map[string]map[string]int) []invRow {
	var rows []invRow
	for _, g := range sidebarLayout {
		for _, t := range g.types {
			byStatus := counts[t.typ]
			if len(byStatus) == 0 || t.typ == "convention" || t.typ == "session" {
				continue
			}
			row := invRow{Label: strings.ToLower(t.label), Href: "/type/" + t.typ + "?project=" + project}
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
				row.Segs = append(row.Segs, invSeg{Class: barClass(t.typ, st), N: n})
				row.Statuses = append(row.Statuses, invStatus{Type: t.typ, Status: st, Glyph: glyphs[st], N: n})
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// fillLately fills the prose bands from the newest decisions, learnings and open threads.
func (s *server) fillLately(p *projectPage) error {
	var err error
	if p.Decided, err = s.newest("decision", index.QueryFilters{Project: p.Name}); err != nil {
		return err
	}
	if p.Learned, err = s.newest("learning", index.QueryFilters{Project: p.Name}); err != nil {
		return err
	}
	drafts, err := s.db.ListByType("learning", index.QueryFilters{Project: p.Name, Status: "draft"})
	if err != nil {
		return err
	}
	p.LearnedDrafts = len(drafts)
	p.OpenThreads, err = s.newest("thread", index.QueryFilters{Project: p.Name, Status: "open"})
	return err
}

// newest returns the lately newest rows of typ, newest updated first.
func (s *server) newest(typ string, f index.QueryFilters) ([]proseItem, error) {
	rows, err := s.newestRows(typ, f, lately)
	if err != nil {
		return nil, fmt.Errorf("newest %s: %w", typ, err)
	}
	var out []proseItem
	for _, r := range rows {
		out = append(out, proseItem{leaf(r), r.Updated})
	}
	return out, nil
}

// acRow is one row of a milestone's Status acceptance table.
type acRow struct {
	N, Text, Measured, Class, Glyph, Met string
}

type flight struct {
	proseItem
	Judge      []prop
	Acceptance []acRow
	Issues     []node
	Resolved   int
	Total      int
}

type doneRow struct {
	proseItem
	Resolved, Total int
	SHA             string
}

type designs struct {
	Canvases []canvas
	Product  []node
	System   []node
	Comps    []node
}

// fillFlight fills the in-progress milestones whole and the six newest done ones.
func (s *server) fillFlight(p *projectPage) error {
	live, err := s.db.ListByType("milestone", index.QueryFilters{Project: p.Name, Status: "in-progress"})
	if err != nil {
		return err
	}
	issues, err := s.db.ListByType("issue", index.QueryFilters{Project: p.Name})
	if err != nil {
		return err
	}
	byID := map[string]index.ArtifactRow{}
	for _, i := range issues {
		byID[i.ID] = i
	}
	for _, r := range live {
		_, art, err := s.load(r.ID)
		if err != nil {
			return err
		}
		f := flight{proseItem: proseItem{leaf(r), r.Updated}, Judge: s.judge("milestone", art.FrontMatter, art.Body), Acceptance: acceptance(art.Body)}
		kids, err := s.milestoneIssues(r.ID, byID)
		if err != nil {
			return err
		}
		f.Total = len(kids)
		for _, k := range kids {
			if k.Status == "resolved" {
				f.Resolved++
			} else if k.Status != "abandoned" && len(f.Issues) < treeCap {
				f.Issues = append(f.Issues, k)
			}
		}
		p.Flight = append(p.Flight, f)
	}
	done, err := s.newestRows("milestone", index.QueryFilters{Project: p.Name, Status: "done"}, 6)
	if err != nil {
		return err
	}
	for _, r := range done {
		_, art, err := s.load(r.ID)
		if err != nil {
			return err
		}
		ms, err := s.db.MilestoneStatus(r.ID)
		if err != nil {
			return err
		}
		p.Done = append(p.Done, doneRow{proseItem{leaf(r), r.Updated}, ms.Resolved, ms.Total, measuredSHA(art.Body)})
	}
	return nil
}

// fillDesigns fills the designs strip: every diagram of the project's product and system designs, then the spine links.
func (s *server) fillDesigns(p *projectPage) error {
	for _, typ := range []string{"product-design", "system-design", "component-design"} {
		rows, err := s.db.ListByType(typ, index.QueryFilters{Project: p.Name})
		if err != nil {
			return err
		}
		for _, r := range rows {
			switch typ {
			case "product-design":
				p.Designs.Product = append(p.Designs.Product, leaf(r))
			case "system-design":
				p.Designs.System = append(p.Designs.System, leaf(r))
			default:
				p.Designs.Comps = append(p.Designs.Comps, leaf(r))
			}
			if typ == "component-design" {
				continue
			}
			_, art, err := s.load(r.ID)
			if err != nil {
				return err
			}
			if typ == "product-design" && p.Deck == "" {
				p.Deck, _ = art.FrontMatter["description"].(string)
			}
			for _, c := range diagramsOf(art.FrontMatter) {
				if !slices.Contains(p.Designs.Canvases, c) {
					p.Designs.Canvases = append(p.Designs.Canvases, c)
				}
			}
		}
	}
	return nil
}

func (s *server) newestRows(typ string, f index.QueryFilters, n int) ([]index.ArtifactRow, error) {
	rows, err := s.db.ListByType(typ, f)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(rows, func(a, b index.ArtifactRow) int { return strings.Compare(b.Updated, a.Updated) })
	return rows[:min(len(rows), n)], nil
}

var shaRe = regexp.MustCompile(`\bat ([0-9a-f]{7,40})\b`)

// measuredSHA returns the commit in a milestone's last Measured line, or "".
func measuredSHA(body string) string {
	if m := shaRe.FindStringSubmatch(lastMeasured(body)); m != nil {
		return m[1]
	}
	return ""
}

// acceptance parses the Status section's table. Cells are read from the right
// because an acceptance cell may hold a command with a pipe.
func acceptance(body string) []acRow {
	var out []acRow
	in := false
	for line := range strings.SplitSeq(body, "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			in = strings.TrimSpace(h) == "Status"
			continue
		}
		if !in || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 4 {
			continue
		}
		n := strings.TrimSpace(cells[0])
		if n == "" || strings.Trim(n, "0123456789") != "" {
			continue
		}
		met := strings.TrimSpace(cells[len(cells)-2])
		row := acRow{
			N:        n,
			Text:     plain(strings.Join(cells[1:len(cells)-2], "|")),
			Measured: plain(cells[len(cells)-1]),
			Met:      met,
		}
		switch met {
		case "met":
			row.Class, row.Glyph = "status-done", "✓"
		case "not met":
			row.Class, row.Glyph = "status-not-met", "✕"
		}
		out = append(out, row)
	}
	return out
}

// plain drops code ticks and trims: the table shows text, never a runnable command.
func plain(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "`", "")) }
