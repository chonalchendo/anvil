package ui

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// prop is a named value list. Rich carries judge fragments a link cannot.
type prop struct {
	Name   string
	Values []link
	Rich   []part
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
	// Issues is set on milestone pages only: the milestone's issue table.
	Issues []issueRow
	// Tabs is set on issue pages only: hydrate is issue-only.
	Tabs tabs
}

// view is one request's server: its resolver and renderer read a single
// id → row catalog, so no link costs an index query.
type view struct {
	*server
	res resolver
	md  markdown
}

// catalogAll is the row limit that reads the whole index.
const catalogAll = math.MaxInt32

func (s *server) view() (*view, error) {
	// RecentlyUpdated omits sessions; the only other read, ListByType, costs a
	// second query per page and a session is never a link target worth a title.
	all, err := s.db.RecentlyUpdated(catalogAll)
	if err != nil {
		return nil, fmt.Errorf("reading catalog: %w", err)
	}
	rows := make(map[string]index.ArtifactRow, len(all))
	for _, r := range all {
		rows[r.ID] = r
	}
	res := resolver{v: s.v, rows: rows}
	return &view{server: s, res: res, md: newMarkdown(res)}, nil
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
	vw, err := s.view()
	if err != nil {
		slog.Error("building artifact page", "key", key, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	page, err := vw.buildArtifact(key, art)
	if err != nil {
		slog.Error("building artifact page", "key", key, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "artifact", page)
}

func (s *view) buildArtifact(key string, art *core.Artifact) (artifactPage, error) {
	pb, err := s.md.renderPage(art.Body, s.res)
	if err != nil {
		return artifactPage{}, err
	}
	title, _ := art.FrontMatter["title"].(string)
	if title == "" {
		title = key
	}
	page := artifactPage{Title: title, Key: key, Head: s.header(key, art.FrontMatter, art.Body), Body: pb.HTML}
	in, err := s.db.LinksTo(key)
	if err != nil {
		return artifactPage{}, fmt.Errorf("incoming links: %w", err)
	}
	cited := s.cited(key, in)
	var tb tabs
	if typeOfKey(key) == string(core.TypeIssue) {
		tb = issueTabs(key, "issue")
	}
	page.Tabs = tb
	page.Diagrams = diagramsOf(art.FrontMatter)
	page.Crumbs = s.crumbs(key, page.Head.Project)
	if page.Head.Type == string(core.TypeMilestone) {
		if page.Issues, err = s.milestoneIssues(key, page.Head.Project, in); err != nil {
			return artifactPage{}, fmt.Errorf("milestone issues: %w", err)
		}
	}
	page.Props = s.props(typeOfKey(key), art.FrontMatter)
	page.Outline, page.Links = pb.Outline, pb.Links
	if len(page.Props) > 0 {
		page.Outline = append(page.Outline, outlineItem{N: len(page.Outline) + 1, Title: "All properties", ID: "props"})
	}
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

func (s *view) header(key string, fm map[string]any, body string) header {
	str := func(k string) string { v, _ := fm[k].(string); return v }
	h := header{Type: typeOfKey(key), Status: str("status"), Project: str("project"), Updated: str("updated"), Description: str("description")}
	h.Glyph = glyphs[h.Status]
	h.Icon = typeIcons[h.Type]
	h.Judge = s.res.judge(h.Type, fm, body)
	for _, n := range headerSlots {
		if v, ok := fm[n]; ok {
			h.Slots = append(h.Slots, s.res.prop(n, v))
		}
	}
	return h
}

// unshownKeys are frontmatter fields no reader job needs: neither the judge strip
// nor "All properties" prints them (decision anvil-human-view.0004).
var unshownKeys = map[string][]string{"learning": {"diataxis"}, "decision": {"date"}}

// props lists the frontmatter the header does not show, in name order; a
// `[[type.id]]` value becomes a link.
func (s *view) props(typ string, fm map[string]any) []prop {
	names := make([]string, 0, len(fm))
	for n := range fm {
		if !headerKeys[n] && !slices.Contains(headerSlots, n) && !slices.Contains(judgeKeys[typ], n) && !slices.Contains(unshownKeys[typ], n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]prop, 0, len(names))
	for _, n := range names {
		v, ok := judgedValue(typ, n, fm[n])
		if !ok {
			continue
		}
		out = append(out, s.res.prop(n, v))
	}
	return out
}

func (r resolver) prop(name string, v any) prop {
	p := prop{Name: name}
	if list, ok := v.([]any); ok {
		for _, e := range list {
			p.Values = append(p.Values, r.slotValue(e))
		}
		return p
	}
	p.Values = []link{r.slotValue(v)}
	return p
}

func (r resolver) slotValue(v any) link {
	str := fmt.Sprint(v)
	if inner := core.UnwrapWikilink(str); inner != str {
		return r.resolve(inner)
	}
	return link{Text: str, Plain: true}
}

// crumbs climbs the spine slots from key through the index, nearest ancestor
// last. The index already holds every parent edge, so no parent file is read.
// An issue climbs one step, to its milestone.
func (s *view) crumbs(key, project string) []link {
	var up []link
	for range maxCrumbs {
		next, ok := s.parent(key)
		if !ok {
			break
		}
		if typeOfKey(key) == string(core.TypeIssue) {
			if _, known := s.res.rows[next]; !known {
				next = milestoneKey(project, next)
			}
		}
		l := s.res.resolve(next)
		up = append(up, l)
		if l.Href == "" || typeOfKey(key) == string(core.TypeIssue) {
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
func (s *view) cited(key string, rows []index.LinkRow) []citedGroup {
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
		sort.Slice(srcs, func(a, b int) bool {
			if ua, ub := s.res.rows[srcs[a]].Updated, s.res.rows[srcs[b]].Updated; ua != ub {
				return ua > ub
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
	return out
}
