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

// lately caps the open threads named in the Threads band.
const lately = 5

// proseItem is a node with the short date it was last updated, for the prose bands.
// Sep is the text that joins it to the item before it in a sentence.
// Note follows the title; an empty Updated prints no date.
type proseItem struct {
	node
	Updated, UpdatedISO, Sep, Note string
}

func proseOf(r index.ArtifactRow) proseItem {
	return proseItem{node: leaf(r), Updated: shortDate(r.Updated), UpdatedISO: day(r.Updated)}
}

type projectPage struct {
	Name, Deck      string
	Conventions     string
	ConventionsVerb string
	Counts          []proseItem
	Milestones      milestonesPanel
	Done            []doneRow
	Designs         designs
	Decided         []paragraph
	Learned         []paragraph
	OpenThreads     []proseItem
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
	page := projectPage{Name: name}
	all, err := s.db.CountByType()
	if err != nil {
		return page, err
	}
	page.Conventions = plural(all["convention"], "convention", "conventions")
	page.ConventionsVerb = pluralWord(all["convention"], "is", "are")
	threads, err := s.projectThreads(name)
	if err != nil {
		return page, err
	}
	// Threads carry no project, so the index count is replaced by the topic match.
	counts["thread"] = map[string]int{}
	for _, t := range threads {
		counts["thread"][t.Status]++
		if t.Status == "open" && len(page.OpenThreads) < lately {
			page.OpenThreads = append(page.OpenThreads, proseOf(t))
		}
	}
	joinProse(page.OpenThreads)
	page.Counts = countParts(name, counts)
	if err := s.fillDesigns(&page, counts); err != nil {
		return page, err
	}
	ms, err := s.db.ListByType("milestone", index.QueryFilters{Project: name})
	if err != nil {
		return page, err
	}
	issues, err := s.db.ListByType("issue", index.QueryFilters{Project: name})
	if err != nil {
		return page, err
	}
	members, err := s.milestoneMembers(name, issues, ms)
	if err != nil {
		return page, err
	}
	if err := s.fillMilestones(&page, ms, issues, members, page.Designs.Product); err != nil {
		return page, err
	}
	if err := s.fillDone(&page, counts, members); err != nil {
		return page, err
	}
	return page, s.fillLately(&page, counts)
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

// countParts lists the project's artifact counts as links to the typed lists, the three design
// types merged; conventions are shared and sessions are not state.
func countParts(project string, counts map[string]map[string]int) []proseItem {
	var out []proseItem
	add := func(n int, one, typ string) {
		if n > 0 {
			out = append(out, proseItem{node: node{Href: "/type/" + typ + "?project=" + url.QueryEscape(project), Title: plural(n, one, one+"s")}})
		}
	}
	var designs int
	first := ""
	for _, t := range []string{"product-design", "system-design", "component-design"} {
		n := sumCounts(counts[t])
		if designs += n; n > 0 && first == "" {
			first = t
		}
	}
	add(designs, "design", first)
	for _, t := range []struct{ typ, label string }{{"milestone", "milestone"}, {"issue", "issue"}, {"decision", "decision"}, {"learning", "learning"}} {
		add(sumCounts(counts[t.typ]), t.label, t.typ)
	}
	if n := sumCounts(counts["thread"]); n > 0 {
		out = append(out, proseItem{node: node{Href: "/type/thread", Title: plural(n, "thread", "threads")}})
	}
	joinProse(out)
	return out
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

func sumCounts(m map[string]int) int {
	t := 0
	for _, n := range m {
		t += n
	}
	return t
}
