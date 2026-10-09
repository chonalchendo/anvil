package ui

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
)

type sidebarType struct {
	Type  string
	Count int
}

type sidebarGroup struct {
	Name  string
	Types []sidebarType
}

type projectLink struct{ Name, Href string }

// sidebar is the nav data every page renders; it comes from the index only.
type sidebar struct {
	Groups   []sidebarGroup
	Projects []projectLink
	Port     string
}

// sidebarLayout groups every type by purpose, in display order.
var sidebarLayout = []struct {
	name  string
	types []string
}{
	{"Design", []string{"product-design", "system-design", "component-design"}},
	{"Work", []string{"milestone", "issue"}},
	{"Knowledge", []string{"convention", "decision", "learning", "thread"}},
	{"Capture", []string{"inbox", "session"}},
}

// render builds the sidebar once per request, then renders the page.
func (s *server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	sb, err := s.sidebar(r)
	if err != nil {
		slog.Error("building sidebar", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.pages.render(w, name, sb, data)
}

func (s *server) sidebar(r *http.Request) (sidebar, error) {
	counts, err := s.db.CountByType()
	if err != nil {
		return sidebar{}, err
	}
	projects, err := s.db.Projects()
	if err != nil {
		return sidebar{}, err
	}
	sb := sidebar{}
	for _, g := range sidebarLayout {
		grp := sidebarGroup{Name: g.name}
		for _, t := range g.types {
			grp.Types = append(grp.Types, sidebarType{Type: t, Count: counts[t]})
		}
		sb.Groups = append(sb.Groups, grp)
	}
	for _, p := range projects {
		sb.Projects = append(sb.Projects, projectLink{Name: p, Href: "/type/issue?project=" + url.QueryEscape(p)})
	}
	// The port comes from the request so Handler needs no listener knowledge.
	if _, port, err := net.SplitHostPort(r.Host); err == nil {
		sb.Port = port
	}
	return sb, nil
}
