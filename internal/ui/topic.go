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
	Ord, Description, Moved, MovedISO, Route string
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
	learnings, err := s.db.ListByType("learning", index.QueryFilters{})
	if err != nil {
		return page, err
	}
	raw, err := s.db.ListByType("inbox", index.QueryFilters{Status: "raw"})
	if err != nil {
		return page, err
	}
	linked, cited, err := s.splitCiting(members, learnings, raw)
	if err != nil {
		return page, err
	}
	page.Linked = plainItems(linked)
	if page.Tags, err = s.domainTags(members); err != nil {
		return page, err
	}
	if page.Tagged, page.TaggedMore, page.TaggedMoreHref, err = s.taggedLearnings(page.Tags, learnings, linked); err != nil {
		return page, err
	}
	if page.Inbox, err = s.topicInbox(t.Slug, cited); err != nil {
		return page, err
	}
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

// splitCiting splits the learnings and raw inbox notes that link a member by their rows in the lists in hand,
// with one index read for the incoming edges.
func (s *server) splitCiting(members, learnings, raw []index.ArtifactRow) (linked, cited []index.ArtifactRow, err error) {
	byID := map[string]index.ArtifactRow{}
	for _, r := range slices.Concat(learnings, raw) {
		byID[r.ID] = r
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	in, err := s.db.LinksToAny(ids)
	if err != nil {
		return nil, nil, err
	}
	for _, l := range in {
		row, ok := byID[l.Source]
		switch {
		case !ok:
		case row.Type == "learning":
			linked = append(linked, row)
		default:
			cited = append(cited, row)
		}
	}
	return dedupe(linked), dedupe(cited), nil
}

// topicInbox lists the raw notes that link a member or name the slug as a whole word, newest first, with their Route.
// The FTS prefilter finds candidates; only their files are read to confirm the word. A "/" joins a word to a path
// segment, so internal/cli/x does not name the topic cli.
func (s *server) topicInbox(slug string, cited []index.ArtifactRow) ([]topicItem, error) {
	hits, err := s.db.Search(slug, 0)
	if err != nil {
		return nil, err
	}
	word := regexp.MustCompile(`(?i)(?:^|[^\w/-])` + regexp.QuoteMeta(slug) + `(?:$|[^\w/-])`)
	routes := map[string]string{}
	rows := slices.Clone(cited)
	for _, r := range cited {
		_, art, err := s.load(r.ID)
		if err != nil {
			return nil, err
		}
		routes[r.ID] = routeOf(art.Body)
	}
	for _, h := range hits {
		if h.Type != "inbox" || h.Status != "raw" {
			continue
		}
		if _, ok := routes[h.ID]; ok {
			continue
		}
		_, art, err := s.load(h.ID)
		if err != nil {
			return nil, err
		}
		if word.MatchString(h.Title + "\n" + art.Body) {
			rows = append(rows, h.ArtifactRow)
			routes[h.ID] = routeOf(art.Body)
		}
	}
	slices.SortStableFunc(rows, byNewest)
	items := plainItems(rows)
	for i, r := range rows {
		items[i].Route = routes[r.ID]
	}
	return items, nil
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

// taggedLearnings lists up to tagCap learnings that share a domain tag with the topic and are not already linked.
// The "N more" link needs one tag: with several, no single tag filter lists them.
func (s *server) taggedLearnings(domainTags []string, learnings, linked []index.ArtifactRow) (items []topicItem, more int, href string, err error) {
	if len(domainTags) == 0 {
		return nil, 0, "", nil
	}
	tags, err := s.db.TagsByType("learning")
	if err != nil {
		return nil, 0, "", err
	}
	skip := map[string]bool{}
	for _, l := range linked {
		skip[l.ID] = true
	}
	rows := slices.DeleteFunc(slices.Clone(learnings), func(r index.ArtifactRow) bool {
		return skip[r.ID] || !slices.ContainsFunc(tags[r.ID], func(tag string) bool { return slices.Contains(domainTags, tag) })
	})
	all := plainItems(rows)
	items = all[:min(len(all), tagCap)]
	if len(domainTags) == 1 {
		href = "/type/learning?tag=" + url.QueryEscape(domainTags[0])
	}
	return items, len(all) - len(items), href, nil
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
// With neither, it names the topic's newest row.
func (s *server) stands(t *topic) (string, error) {
	var desc string
	if d, ok := t.newestLive(); ok {
		_, art, err := s.load(d.ID)
		if err != nil {
			return "", err
		}
		desc, _ = art.FrontMatter["description"].(string)
		desc = strings.TrimSuffix(strings.TrimSpace(desc), ".")
	}
	threads := slices.Clone(t.Threads)
	slices.SortStableFunc(threads, byNewest)
	for _, th := range threads {
		if th.Status != "open" {
			continue
		}
		if desc == "" {
			return "Open: " + th.Title, nil
		}
		if strings.HasSuffix(desc, "?") || strings.HasSuffix(desc, "!") {
			return desc + " Open: " + th.Title, nil
		}
		return desc + "; open: " + th.Title, nil
	}
	if desc != "" {
		return sentence(desc), nil
	}
	return "Nothing current; newest: " + sentence(t.newest().Title), nil
}

// sentence closes text with a full stop unless it already ends in terminal punctuation.
func sentence(text string) string {
	if strings.HasSuffix(text, ".") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, "!") {
		return text
	}
	return text + "."
}

// newest is the topic's newest row across decisions and threads.
func (t *topic) newest() index.ArtifactRow {
	rows := slices.Concat(t.Decisions, t.Threads)
	slices.SortStableFunc(rows, byNewest)
	return rows[0]
}
