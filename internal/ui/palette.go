package ui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// paletteEntry is the shape the ⌘K script in base.html reads.
type paletteEntry struct {
	Key    string `json:"key"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Href   string `json:"href"`

	moved string
}

// paletteWeight ranks entry types, highest first; sessions are absent so the palette drops them.
var paletteWeight = []string{
	"project", "milestone", "issue", "product-design", "system-design", "component-design",
	"topic", "decision", "thread", "learning", "convention", "inbox", "sweep",
}

// palette serves projects, topics and every indexed artifact except sessions,
// ranked by type weight then newest first. Failures return a non-OK status so
// the client does not cache a partial list.
func (s *server) palette(w http.ResponseWriter, _ *http.Request) {
	out, err := s.buildPalette()
	if err != nil {
		slog.Error("building palette", "err", err)
		http.Error(w, "palette failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *server) buildPalette() ([]paletteEntry, error) {
	out := []paletteEntry{}
	projects, err := s.db.Projects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		out = append(out, paletteEntry{Key: p, Type: "project", Title: p, Href: "/project/" + url.PathEscape(p)})
	}
	topics, err := s.readTopics()
	if err != nil {
		return nil, err
	}
	for _, t := range topics {
		out = append(out, paletteEntry{Key: t.Slug, Type: "topic", Title: t.Slug, Href: topicHref(t.Slug), moved: t.Moved})
	}
	for _, t := range core.AllTypes {
		if t == core.TypeSession {
			continue
		}
		rows, err := s.db.ListByType(string(t), index.QueryFilters{})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, paletteEntry{Key: r.ID, Type: r.Type, Title: r.Title, Status: r.Status, Href: "/artifact/" + url.PathEscape(r.ID), moved: r.Updated})
		}
	}
	slices.SortStableFunc(out, func(a, b paletteEntry) int {
		if c := slices.Index(paletteWeight, a.Type) - slices.Index(paletteWeight, b.Type); c != 0 {
			return c
		}
		return strings.Compare(b.moved, a.moved)
	})
	return out, nil
}
