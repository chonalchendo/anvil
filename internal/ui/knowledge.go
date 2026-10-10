package ui

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// routeCap bounds the Route line shown for a routed inbox note.
const routeCap = 200

type topicRow struct {
	Slug, Href, Status, Stands, Moved, MovedISO string
	ThreadsWord                                 string
	Decisions, Threads                          []statusN
}

type otherRow struct {
	Href, Slug, Status, Moved, MovedISO string
}

type routedRow struct {
	node
	Moved, MovedISO, Route string
}

type knowledgePage struct {
	Counts     []proseItem
	TopicsLede string
	Topics     []topicRow
	Other      []otherRow
	RoutedLede string
	Routed     []routedRow
}

// knowledge serves the vault-wide topic page at /.
func (s *server) knowledge(w http.ResponseWriter, r *http.Request) {
	page, err := s.buildKnowledge()
	if err != nil {
		slog.Error("building knowledge page", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "knowledge", page)
}

func (s *server) buildKnowledge() (knowledgePage, error) {
	topics, err := s.readTopics()
	if err != nil {
		return knowledgePage{}, err
	}
	raw, err := s.db.ListByType("inbox", index.QueryFilters{Status: "raw"})
	if err != nil {
		return knowledgePage{}, err
	}
	counts, err := s.db.CountByType()
	if err != nil {
		return knowledgePage{}, err
	}
	page := knowledgePage{Counts: []proseItem{
		countLink("/type/decision", counts["decision"], "decision", "decisions"),
		countLink("/type/learning", counts["learning"], "learning", "learnings"),
		countLink("/type/thread", counts["thread"], "thread", "threads"),
		countLink("/type/inbox?status=raw", len(raw), "raw inbox note", "raw inbox notes"),
	}}
	joinProse(page.Counts)

	var decisions, threads, proposed, open int
	for _, t := range sortedTopics(topics) {
		decisions += len(t.Decisions)
		threads += len(t.Threads)
		for _, d := range t.Decisions {
			if d.Status == "proposed" {
				proposed++
			}
		}
		for _, th := range t.Threads {
			if th.Status == "open" {
				open++
			}
		}
		if len(t.Threads) == 0 && len(t.Decisions) == 1 {
			d := t.Decisions[0]
			page.Other = append(page.Other, otherRow{Href: topicHref(t.Slug), Slug: t.Slug, Status: d.Status, Moved: shortDate(t.Moved), MovedISO: t.Moved})
			continue
		}
		stands, err := s.stands(t)
		if err != nil {
			return knowledgePage{}, err
		}
		page.Topics = append(page.Topics, topicRow{
			Slug: t.Slug, Href: topicHref(t.Slug), Status: t.status(), ThreadsWord: pluralWord(len(t.Threads), "thread", "threads"), Stands: stands,
			Moved: shortDate(t.Moved), MovedISO: t.Moved, Decisions: tally(t.Decisions), Threads: tally(t.Threads),
		})
	}
	page.TopicsLede = topicsLede(len(topics), decisions, threads, proposed, open, len(page.Topics), len(page.Other))
	if page.Routed, err = s.routedInbox(raw); err != nil {
		return knowledgePage{}, err
	}
	page.RoutedLede = fmt.Sprintf("%d of %s %s a route", len(page.Routed), plural(len(raw), "raw note", "raw notes"), pluralWord(len(page.Routed), "carries", "carry"))
	if len(page.Routed) > 0 {
		page.RoutedLede += ", newest first"
	}
	page.RoutedLede += "."
	return page, nil
}

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func countLink(href string, n int, one, many string) proseItem {
	return proseItem{node: node{Href: href, Title: plural(n, one, many)}}
}

func topicHref(slug string) string { return "/topic/" + url.PathEscape(slug) }

func projectHref(slug string) string { return "/project/" + url.PathEscape(slug) }

// status is the status that colours the topic name: the newest live decision's, else the newest row's.
func (t *topic) status() string {
	if d, ok := t.newestLive(); ok {
		return d.Status
	}
	return t.newest().Status
}

// sortedTopics orders topics by last movement, newest first.
func sortedTopics(m map[string]*topic) []*topic {
	out := make([]*topic, 0, len(m))
	for _, t := range m {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b *topic) int {
		if c := strings.Compare(b.Moved, a.Moved); c != 0 {
			return c
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	return out
}

func topicsLede(topics, decisions, threads, proposed, open, listed, folded int) string {
	const noTopic = "Learnings and inbox notes carry no topic."
	if topics == 0 {
		return noTopic
	}
	lede := fmt.Sprintf("%s, read from the ids of %s and %s; %s proposed and %s open.",
		plural(topics, "topic", "topics"), plural(decisions, "decision", "decisions"), plural(threads, "thread", "threads"),
		plural(proposed, "decision is", "decisions are"), plural(open, "thread", "threads"))
	if listed > 0 {
		lede += fmt.Sprintf(" The %s with a thread or more than one decision %s below, newest movement first.",
			plural(listed, "topic", "topics"), pluralWord(listed, "is", "are"))
	}
	switch {
	case folded == 1 && listed > 0:
		lede += " The other topic holds one decision and folds at the end."
	case folded == 1:
		lede += " It holds one decision and folds below."
	case folded > 1 && listed == 0:
		lede += " They hold one decision each and fold below."
	case folded > 1:
		lede += fmt.Sprintf(" The other %s hold one decision each and fold at the end.", plural(folded, "topic", "topics"))
	}
	return lede + " " + noTopic
}

// routedInbox lists the raw inbox notes with a `## Route` section, newest first. The FTS index prefilters on the word;
// only those files are read to confirm the heading.
func (s *server) routedInbox(raw []index.ArtifactRow) ([]routedRow, error) {
	hits, err := s.db.Search("Route", 0)
	if err != nil {
		return nil, err
	}
	isRaw := map[string]bool{}
	for _, r := range raw {
		isRaw[r.ID] = true
	}
	var rows []index.ArtifactRow
	for _, h := range hits {
		if isRaw[h.ID] {
			rows = append(rows, h.ArtifactRow)
		}
	}
	slices.SortStableFunc(rows, byNewest)
	var out []routedRow
	for _, r := range rows {
		_, art, err := s.load(r.ID)
		if err != nil {
			return nil, err
		}
		route := routeOf(art.Body)
		if route == "" {
			continue
		}
		out = append(out, routedRow{node: leaf(r), Moved: shortDate(r.Updated), MovedISO: day(r.Updated), Route: route})
	}
	return out, nil
}

// routeOf is the first line of a note's `## Route` section, without markdown backticks, capped at routeCap.
func routeOf(body string) string {
	route, _, _ := strings.Cut(core.Section(body, "Route"), "\n")
	route = strings.TrimSpace(strings.ReplaceAll(route, "`", ""))
	if rs := []rune(route); len(rs) > routeCap {
		route = string(rs[:routeCap-1]) + "…"
	}
	return route
}
