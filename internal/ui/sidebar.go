package ui

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
)

type sidebarType struct {
	Type, Icon string
	Label      string
	Count      string
}

type sidebarGroup struct {
	Name  string
	Types []sidebarType
}

type projectLink struct {
	Name, Href string
	Current    bool
}

// sidebar is the nav data every page renders; it comes from the index only.
type sidebar struct {
	Groups   []sidebarGroup
	Projects []projectLink
	Port     string
}

type sidebarEntry struct{ typ, label string }

// sidebarLayout groups every type by purpose, in display order.
var sidebarLayout = []struct {
	name  string
	types []sidebarEntry
}{
	{"Design", []sidebarEntry{{"product-design", "Product designs"}, {"system-design", "System designs"}, {"component-design", "Component designs"}}},
	{"Work", []sidebarEntry{{"milestone", "Milestones"}, {"issue", "Issues"}}},
	{"Knowledge", []sidebarEntry{{"convention", "Conventions"}, {"decision", "Decisions"}, {"learning", "Learnings"}, {"thread", "Threads"}}},
	{"Capture", []sidebarEntry{{"inbox", "Inbox"}, {"session", "Sessions"}}},
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
			grp.Types = append(grp.Types, sidebarType{Type: t.typ, Icon: typeIcons[t.typ], Label: t.label, Count: groupThousands(counts[t.typ])})
		}
		sb.Groups = append(sb.Groups, grp)
	}
	for _, p := range projects {
		sb.Projects = append(sb.Projects, projectLink{Name: p, Href: "/project/" + url.PathEscape(p), Current: p == r.PathValue("slug")})
	}
	// The port comes from the request so Handler needs no listener knowledge.
	if _, port, err := net.SplitHostPort(r.Host); err == nil {
		sb.Port = port
	}
	return sb, nil
}

// groupThousands formats n with a comma between each group of three digits.
func groupThousands(n int) string {
	d := strconv.Itoa(n)
	for i := len(d) - 3; i > 0; i -= 3 {
		d = d[:i] + "," + d[i:]
	}
	return d
}
