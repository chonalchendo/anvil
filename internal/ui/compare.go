package ui

import (
	"errors"
	"log/slog"
	"net/http"
)

type comparePage struct {
	Title string
	Panes [2]artifactPage
}

// compare renders two nodes side by side. It is stateless: both keys come
// from the query, and a == b shows the same node twice.
func (s *server) compare(w http.ResponseWriter, r *http.Request) {
	var page comparePage
	for i, name := range [2]string{"a", "b"} {
		key, art, err := s.load(r.URL.Query().Get(name))
		if errors.Is(err, errNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			slog.Error("loading artifact", "param", name, "err", err)
			http.Error(w, "artifact unreadable", http.StatusInternalServerError)
			return
		}
		page.Panes[i], err = s.node(key, art)
		if err != nil {
			slog.Error("building compare pane", "param", name, "key", key, "err", err)
			http.Error(w, "page failed", http.StatusInternalServerError)
			return
		}
	}
	page.Title = page.Panes[0].Title + " vs " + page.Panes[1].Title
	s.render(w, r, "compare", page)
}
