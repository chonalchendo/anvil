package ui

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

type prop struct {
	Name   string
	Values []link
}

// group is a run of links sharing a relation (and, for incoming links, a source type).
type group struct {
	Relation, SourceType string
	Items                []link
}

type artifactPage struct {
	Title, Key string
	Crumbs     []link
	Props      []prop
	Body       template.HTML
	Hanging    []group
	Out        []group
}

// spineSlots are the frontmatter slots a breadcrumb climbs, in preference order.
var spineSlots = []string{"milestone", "system_design", "product_design"}

const maxCrumbs = 6

func (s *server) artifact(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	prefix, id, ok := strings.Cut(key, ".")
	t, err := core.ParseType(prefix)
	if !ok || err != nil || id == "" || strings.ContainsAny(id, `/\`) {
		http.NotFound(w, r)
		return
	}
	cid, path, err := core.ResolveArtifact(s.v, t, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	art, err := core.LoadArtifact(path)
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("loading artifact", "key", key, "err", err)
		http.Error(w, "artifact unreadable", http.StatusInternalServerError)
		return
	}
	page, err := s.buildArtifact(core.IndexKey(t, cid), art)
	if err != nil {
		slog.Error("building artifact page", "key", key, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.pages.render(w, "artifact", page)
}

func (s *server) buildArtifact(key string, art *core.Artifact) (artifactPage, error) {
	body, err := s.md.render(art.Body)
	if err != nil {
		return artifactPage{}, err
	}
	title, _ := art.FrontMatter["title"].(string)
	if title == "" {
		title = key
	}
	in, err := s.db.LinksTo(key)
	if err != nil {
		return artifactPage{}, fmt.Errorf("incoming links: %w", err)
	}
	out, err := s.db.LinksFrom(key)
	if err != nil {
		return artifactPage{}, fmt.Errorf("outgoing links: %w", err)
	}
	return artifactPage{
		Title:   title,
		Key:     key,
		Crumbs:  s.crumbs(key),
		Props:   s.props(art.FrontMatter),
		Body:    body,
		Hanging: s.groups(in, func(r index.LinkRow) (string, string) { return r.Source, typeOfKey(r.Source) }),
		Out:     s.groups(out, func(r index.LinkRow) (string, string) { return r.Target, "" }),
	}, nil
}

func typeOfKey(key string) string {
	t, _, _ := strings.Cut(key, ".")
	return t
}

// props lists frontmatter in name order; a `[[type.id]]` value becomes a link.
func (s *server) props(fm map[string]any) []prop {
	names := make([]string, 0, len(fm))
	for n := range fm {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]prop, 0, len(names))
	for _, n := range names {
		p := prop{Name: n}
		switch v := fm[n].(type) {
		case []any:
			for _, e := range v {
				p.Values = append(p.Values, s.slotValue(e))
			}
		default:
			p.Values = []link{s.slotValue(v)}
		}
		out = append(out, p)
	}
	return out
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

// groups folds link rows into relation (and source type) groups, sorted by name.
func (s *server) groups(rows []index.LinkRow, pick func(index.LinkRow) (target, srcType string)) []group {
	byName := map[string]*group{}
	var names []string
	for _, r := range rows {
		target, srcType := pick(r)
		name := r.Relation + "\x00" + srcType
		g, ok := byName[name]
		if !ok {
			g = &group{Relation: r.Relation, SourceType: srcType}
			byName[name] = g
			names = append(names, name)
		}
		g.Items = append(g.Items, s.res.resolve(target))
	}
	sort.Strings(names)
	out := make([]group, 0, len(names))
	for _, n := range names {
		out = append(out, *byName[n])
	}
	return out
}
