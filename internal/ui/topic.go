package ui

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/index"
)

// tagCap is how many by-tag learnings the topic page lists before "N more".
const tagCap = 8

// topicItem is one decision, thread, learning or inbox row on the topic page.
type topicItem struct {
	node
	Ord, Description, Moved, MovedISO string
}

type topicPage struct {
	Slug, Lead               string
	DecisionsAll, ThreadsAll string
	Decisions, Threads       []topicItem
	Linked, Tagged, Inbox    []topicItem
	Tags                     []string
	TaggedMore               int
	TaggedMoreHref           string
}

// topic serves /topic/<slug>; a slug no thread or decision id carries is a 404.
func (s *server) topic(w http.ResponseWriter, r *http.Request) {
	topics, err := s.readTopics()
	if err != nil {
		slog.Error("reading topics", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	t := topics[r.PathValue("slug")]
	if t == nil {
		http.NotFound(w, r)
		return
	}
	page, err := s.buildTopic(t)
	if err != nil {
		slog.Error("building topic page", "topic", t.Slug, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "topic", page)
}

func (s *server) buildTopic(t *topic) (topicPage, error) {
	page := topicPage{
		Slug: t.Slug, Lead: topicLead(t),
		DecisionsAll: "/type/decision?topic=" + url.QueryEscape(t.Slug), ThreadsAll: "/type/thread?topic=" + url.QueryEscape(t.Slug),
	}
	var err error
	if page.Decisions, err = s.topicItems(t.Decisions, true); err != nil {
		return page, err
	}
	if page.Threads, err = s.topicItems(t.Threads, false); err != nil {
		return page, err
	}
	members := slices.Concat(t.Decisions, t.Threads)
	linked, err := s.citingRows(members, "learning.")
	if err != nil {
		return page, err
	}
	page.Linked = plainItems(linked)
	if page.Tags, err = s.domainTags(members); err != nil {
		return page, err
	}
	if err := s.fillTagged(&page, linked); err != nil {
		return page, err
	}
	cited, err := s.citingRows(members, "inbox.")
	if err != nil {
		return page, err
	}
	mentions, err := s.db.Search(t.Slug, 0)
	if err != nil {
		return page, err
	}
	for _, h := range mentions {
		if h.Type == "inbox" {
			cited = append(cited, h.ArtifactRow)
		}
	}
	cited = slices.DeleteFunc(cited, func(r index.ArtifactRow) bool { return r.Status != "raw" })
	page.Inbox = plainItems(dedupe(cited))
	return page, nil
}

// topicLead composes the counts and last-moved sentence.
func topicLead(t *topic) string {
	var parts []string
	for _, g := range []struct {
		n    int
		one  string
		many string
		rows []index.ArtifactRow
	}{{len(t.Decisions), "decision", "decisions", t.Decisions}, {len(t.Threads), "thread", "threads", t.Threads}} {
		if g.n > 0 {
			parts = append(parts, fmt.Sprintf("%s (%s)", plural(g.n, g.one, g.many), tallyText(tally(g.rows))))
		}
	}
	return strings.Join(parts, " and ") + ". Last moved " + shortDate(t.Moved) + "."
}

// topicItems lists rows in ordinal order; withDesc reads each description from the file.
func (s *server) topicItems(rows []index.ArtifactRow, withDesc bool) ([]topicItem, error) {
	var out []topicItem
	for _, r := range rows {
		it := topicItem{node: leaf(r), Ord: ordinalOf(r.ID), Moved: shortDate(r.Updated), MovedISO: day(r.Updated)}
		if withDesc {
			_, art, err := s.load(r.ID)
			if err != nil {
				return nil, err
			}
			it.Description, _ = art.FrontMatter["description"].(string)
		}
		out = append(out, it)
	}
	return out, nil
}

// plainItems lists rows newest first, without an ordinal or description.
func plainItems(rows []index.ArtifactRow) []topicItem {
	rows = slices.Clone(rows)
	slices.SortStableFunc(rows, byNewest)
	out := make([]topicItem, len(rows))
	for i, r := range rows {
		out[i] = topicItem{node: leaf(r), Moved: shortDate(r.Updated), MovedISO: day(r.Updated)}
	}
	return out
}

// citingRows returns the artifacts whose id starts with prefix and that link one of members.
func (s *server) citingRows(members []index.ArtifactRow, prefix string) ([]index.ArtifactRow, error) {
	var out []index.ArtifactRow
	for _, m := range members {
		in, err := s.db.LinksTo(m.ID)
		if err != nil {
			return nil, err
		}
		for _, l := range in {
			if !strings.HasPrefix(l.Source, prefix) {
				continue
			}
			row, err := s.db.GetArtifact(l.Source)
			if err != nil {
				return nil, err
			}
			out = append(out, row)
		}
	}
	return dedupe(out), nil
}

func dedupe(rows []index.ArtifactRow) []index.ArtifactRow {
	seen := map[string]bool{}
	return slices.DeleteFunc(slices.Clone(rows), func(r index.ArtifactRow) bool {
		dup := seen[r.ID]
		seen[r.ID] = true
		return dup
	})
}

// domainTags returns the sorted domain/ tags of the members.
func (s *server) domainTags(members []index.ArtifactRow) ([]string, error) {
	set := map[string]bool{}
	for _, typ := range []string{"decision", "thread"} {
		tags, err := s.db.TagsByType(typ)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			if m.Type != typ {
				continue
			}
			for _, tag := range tags[m.ID] {
				if strings.HasPrefix(tag, "domain/") {
					set[tag] = true
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	slices.Sort(out)
	return out, nil
}

// fillTagged lists up to tagCap learnings that share a domain tag with the topic and are not already linked.
func (s *server) fillTagged(page *topicPage, linked []index.ArtifactRow) error {
	if len(page.Tags) == 0 {
		return nil
	}
	rows, err := s.db.ListByType("learning", index.QueryFilters{})
	if err != nil {
		return err
	}
	tags, err := s.db.TagsByType("learning")
	if err != nil {
		return err
	}
	skip := map[string]bool{}
	for _, l := range linked {
		skip[l.ID] = true
	}
	rows = slices.DeleteFunc(rows, func(r index.ArtifactRow) bool {
		return skip[r.ID] || !slices.ContainsFunc(tags[r.ID], func(tag string) bool { return slices.Contains(page.Tags, tag) })
	})
	all := plainItems(rows)
	page.Tagged = all[:min(len(all), tagCap)]
	page.TaggedMore = len(all) - len(page.Tagged)
	page.TaggedMoreHref = "/type/learning"
	if len(page.Tags) == 1 {
		page.TaggedMoreHref += "?tag=" + url.QueryEscape(page.Tags[0])
	}
	return nil
}

// topicIDRe splits a thread or decision id into its topic and ordinal; the index has no topic column.
var topicIDRe = regexp.MustCompile(`^(?:thread|decision)\.(.+?)\.(\d{4})-`)

// topic is the decisions and threads that share an id prefix, each in ordinal order.
type topic struct {
	Slug               string
	Decisions, Threads []index.ArtifactRow
	Moved              string
}

func topicOf(id string) string {
	if m := topicIDRe.FindStringSubmatch(id); m != nil {
		return m[1]
	}
	return ""
}

func ordinalOf(id string) string {
	if m := topicIDRe.FindStringSubmatch(id); m != nil {
		return m[2]
	}
	return ""
}

// readTopics groups every thread and decision row by topic.
func (s *server) readTopics() (map[string]*topic, error) {
	out := map[string]*topic{}
	for _, typ := range []string{"decision", "thread"} {
		rows, err := s.db.ListByType(typ, index.QueryFilters{})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			slug := topicOf(r.ID)
			if slug == "" {
				continue
			}
			t := out[slug]
			if t == nil {
				t = &topic{Slug: slug}
				out[slug] = t
			}
			if typ == "decision" {
				t.Decisions = append(t.Decisions, r)
			} else {
				t.Threads = append(t.Threads, r)
			}
			t.Moved = max(t.Moved, day(r.Updated))
		}
	}
	return out, nil
}

// day cuts an updated stamp to its date.
func day(s string) string { return s[:min(len(s), 10)] }

// tally counts rows per status, live-first.
func tally(rows []index.ArtifactRow) []statusN {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Status]++
	}
	return notDone(counts)
}

// tallyText writes a tally as "1 proposed, 3 accepted".
func tallyText(ns []statusN) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprintf("%d %s", n.N, n.Label)
	}
	return strings.Join(parts, ", ")
}

// newestLive returns the topic's newest decision that is not superseded or rejected.
func (t *topic) newestLive() (index.ArtifactRow, bool) {
	decisions := slices.Clone(t.Decisions)
	slices.SortStableFunc(decisions, byNewest)
	for _, d := range decisions {
		if d.Status != "superseded" && d.Status != "rejected" {
			return d, true
		}
	}
	return index.ArtifactRow{}, false
}

// stands composes the where-it-stands line: the newest live decision's description, then the newest open thread.
func (s *server) stands(t *topic) (string, error) {
	var out string
	if d, ok := t.newestLive(); ok {
		_, art, err := s.load(d.ID)
		if err != nil {
			return "", err
		}
		desc, _ := art.FrontMatter["description"].(string)
		out = strings.TrimSuffix(strings.TrimSpace(desc), ".")
	}
	threads := slices.Clone(t.Threads)
	slices.SortStableFunc(threads, byNewest)
	for _, th := range threads {
		if th.Status != "open" {
			continue
		}
		if out == "" {
			return "Open: " + th.Title, nil
		}
		return out + "; open: " + th.Title, nil
	}
	return out, nil
}
