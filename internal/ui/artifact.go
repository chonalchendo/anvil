package ui

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

type prop struct {
	Name   string
	Values []link
}

// citedGroup is the cited-by fold's run of sources sharing one type. Count is
// the total; Items keeps the first citedMax, MoreHref reaches the rest.
type citedGroup struct {
	Type     string
	Count    int
	Items    []link
	More     int
	MoreHref string
}

// header is the node header: the identity fields, the judge fields and the
// typed slots the state line needs.
type header struct {
	Type, Icon, Status, Glyph, Project, Updated, Description string
	Slots                                                    []prop
	Judge                                                    []prop
}

type artifactPage struct {
	Title, Key string
	Head       header
	Crumbs     []link
	Props      []prop
	Body       template.HTML
	Diagrams   []canvas
	// Outline, Links and Cited fill the contents column; Links is the body's
	// `## Links` section as a sentence.
	Outline    []outlineItem
	Links      template.HTML
	Cited      []citedGroup
	CitedTotal int
	// Tabs is set on issue pages only: hydrate is issue-only.
	Tabs tabs
}

// spineSlots are the frontmatter slots a breadcrumb climbs, in preference order.
var spineSlots = []string{"milestone", "system_design", "product_design"}

const maxCrumbs = 6

var errNotFound = errors.New("artifact not found")

// load resolves a `type.id` key to its canonical index key and artifact.
// A malformed or missing key returns errNotFound.
func (s *server) load(key string) (string, *core.Artifact, error) {
	prefix, id, ok := strings.Cut(key, ".")
	t, err := core.ParseType(prefix)
	if !ok || err != nil || id == "" || strings.ContainsAny(id, `/\`) {
		return "", nil, errNotFound
	}
	cid, path, err := core.ResolveArtifact(s.v, t, id)
	if err != nil {
		return "", nil, errNotFound
	}
	art, err := core.LoadArtifact(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, errNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("loading %s: %w", key, err)
	}
	return core.IndexKey(t, cid), art, nil
}

func (s *server) artifact(w http.ResponseWriter, r *http.Request) {
	key, art, err := s.load(r.PathValue("key"))
	if errors.Is(err, errNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("loading artifact", "err", err)
		http.Error(w, "artifact unreadable", http.StatusInternalServerError)
		return
	}
	page, err := s.buildArtifact(key, art)
	if err != nil {
		slog.Error("building artifact page", "key", key, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "artifact", page)
}

// node fills the part of the page every view of one artifact shares: title,
// key, header and rendered body.
func (s *server) node(key string, art *core.Artifact) (artifactPage, error) {
	body, err := s.md.renderSections(art.Body)
	if err != nil {
		return artifactPage{}, err
	}
	return s.shell(key, art, body), nil
}

// shell is node with the body already rendered.
func (s *server) shell(key string, art *core.Artifact, body template.HTML) artifactPage {
	title, _ := art.FrontMatter["title"].(string)
	if title == "" {
		title = key
	}
	return artifactPage{Title: title, Key: key, Head: s.header(key, art.FrontMatter, art.Body), Body: body}
}

func (s *server) buildArtifact(key string, art *core.Artifact) (artifactPage, error) {
	pb, err := s.md.renderPage(art.Body, s.res)
	if err != nil {
		return artifactPage{}, err
	}
	page := s.shell(key, art, pb.HTML)
	in, err := s.db.LinksTo(key)
	if err != nil {
		return artifactPage{}, fmt.Errorf("incoming links: %w", err)
	}
	cited, err := s.cited(key, in)
	if err != nil {
		return artifactPage{}, err
	}
	var tb tabs
	if typeOfKey(key) == string(core.TypeIssue) {
		tb = issueTabs(key, "issue")
	}
	page.Tabs = tb
	page.Diagrams = diagramsOf(art.FrontMatter)
	page.Crumbs = s.crumbs(key)
	page.Props = s.props(typeOfKey(key), art.FrontMatter)
	page.Outline, page.Links = pb.Outline, pb.Links
	page.Cited = cited
	for _, g := range cited {
		page.CitedTotal += g.Count
	}
	return page, nil
}

func typeOfKey(key string) string {
	t, _, _ := strings.Cut(key, ".")
	return t
}

// headerSlots are the typed slots the state line shows as links. The spine
// slots sit in the breadcrumb and the rest fold into "All properties".
var headerSlots = []string{"depends_on"}

// headerKeys are the frontmatter keys the header shows; props folds the rest.
var headerKeys = map[string]bool{"type": true, "title": true, "status": true, "project": true, "updated": true, "description": true}

func (s *server) header(key string, fm map[string]any, body string) header {
	str := func(k string) string { v, _ := fm[k].(string); return v }
	h := header{Type: typeOfKey(key), Status: str("status"), Project: str("project"), Updated: str("updated"), Description: str("description")}
	h.Glyph = glyphs[h.Status]
	h.Icon = typeIcons[h.Type]
	h.Judge = s.judge(h.Type, fm, body)
	for _, n := range headerSlots {
		if v, ok := fm[n]; ok {
			h.Slots = append(h.Slots, s.prop(n, v))
		}
	}
	return h
}

// props lists the frontmatter the header does not show, in name order; a
// `[[type.id]]` value becomes a link.
func (s *server) props(typ string, fm map[string]any) []prop {
	names := make([]string, 0, len(fm))
	for n := range fm {
		if !headerKeys[n] && !slices.Contains(headerSlots, n) && !slices.Contains(judgeKeys[typ], n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]prop, 0, len(names))
	for _, n := range names {
		out = append(out, s.prop(n, fm[n]))
	}
	return out
}

func (s *server) prop(name string, v any) prop {
	p := prop{Name: name}
	if list, ok := v.([]any); ok {
		for _, e := range list {
			p.Values = append(p.Values, s.slotValue(e))
		}
		return p
	}
	p.Values = []link{s.slotValue(v)}
	return p
}

func (s *server) slotValue(v any) link {
	str := fmt.Sprint(v)
	if inner := core.UnwrapWikilink(str); inner != str {
		return s.res.resolve(inner)
	}
	return link{Text: str, Plain: true}
}

// crumbs climbs the spine slots from key through the index, nearest ancestor
// last. The index already holds every parent edge, so no parent file is read.
func (s *server) crumbs(key string) []link {
	var up []link
	for range maxCrumbs {
		next, ok := s.parent(key)
		if !ok {
			break
		}
		l := s.res.resolve(next)
		up = append(up, l)
		if l.Href == "" {
			break
		}
		key = next
	}
	for i, j := 0, len(up)-1; i < j; i, j = i+1, j-1 {
		up[i], up[j] = up[j], up[i]
	}
	return up
}

// parent returns the target of key's first outgoing spine-slot link.
func (s *server) parent(key string) (string, bool) {
	rows, err := s.db.LinksFrom(key)
	if err != nil {
		return "", false
	}
	t := slotOf(rows, spineSlots...)
	return t, t != ""
}

// slotOf returns the target of the first row whose relation is a slot, trying slots in order.
func slotOf(rows []index.LinkRow, slots ...string) string {
	for _, slot := range slots {
		for _, r := range rows {
			if r.Relation == slot {
				return r.Target
			}
		}
	}
	return ""
}

// citedMax is how many links a cited group shows before "N more".
const citedMax = 8

// cited groups incoming links by source type, one entry per distinct source,
// types in name order, sources newest first. A type with no sources never appears.
func (s *server) cited(key string, rows []index.LinkRow) ([]citedGroup, error) {
	byType := map[string][]string{}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.Source] {
			continue
		}
		seen[r.Source] = true
		t := typeOfKey(r.Source)
		byType[t] = append(byType[t], r.Source)
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)
	out := make([]citedGroup, 0, len(types))
	for _, t := range types {
		srcs := byType[t]
		arts, err := s.db.ListByType(t, index.QueryFilters{})
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", t, err)
		}
		updated := make(map[string]string, len(arts))
		for _, r := range arts {
			updated[r.ID] = r.Updated
		}
		sort.Slice(srcs, func(a, b int) bool {
			if updated[srcs[a]] != updated[srcs[b]] {
				return updated[srcs[a]] > updated[srcs[b]]
			}
			return srcs[a] < srcs[b]
		})
		g := citedGroup{Type: t, Count: len(srcs)}
		for _, src := range srcs[:min(len(srcs), citedMax)] {
			g.Items = append(g.Items, s.res.resolve(src))
		}
		if g.More = len(srcs) - len(g.Items); g.More > 0 {
			g.MoreHref = "/type/" + url.PathEscape(t) + "?to=" + url.QueryEscape(key)
		}
		out = append(out, g)
	}
	return out, nil
}
