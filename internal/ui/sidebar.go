package ui

import (
	"net/http"
	"strings"
)

type projectLink struct {
	Name, Href string
	Current    bool
}

// sidebar is the nav data every page renders; it comes from the index only.
type sidebar struct {
	Projects []projectLink
	// Knowledge is the Knowledge link's aria-current value: "page" on the knowledge page, "true" on a topic page, else empty.
	Knowledge string
}

func (s *server) sidebar(r *http.Request) (sidebar, error) {
	projects, err := s.db.Projects()
	if err != nil {
		return sidebar{}, err
	}
	sb := sidebar{}
	switch {
	case r.URL.Path == "/":
		sb.Knowledge = "page"
	case strings.HasPrefix(r.URL.Path, "/topic/"):
		sb.Knowledge = "true"
	}
	for _, p := range projects {
		sb.Projects = append(sb.Projects, projectLink{Name: p, Href: projectHref(p), Current: p == r.PathValue("slug")})
	}
	return sb, nil
}
