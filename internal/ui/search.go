package ui

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"
)

// searchCap bounds the hits one /search page renders.
const searchCap = 50

type searchRow struct {
	Href, ID, Title, Status, Glyph string
	Snippet                        template.HTML
}

type searchGroup struct {
	Type, Icon string
	Hits       []searchRow
}

type searchPage struct {
	Query    string
	Total    int
	Groups   []searchGroup
	Searched bool
}

// search serves body hits from the index FTS, grouped by type in the order of
// each type's best-ranked hit. An empty q renders the form only.
func (s *server) search(w http.ResponseWriter, r *http.Request) {
	page := searchPage{Query: strings.TrimSpace(r.URL.Query().Get("q"))}
	if page.Query != "" {
		hits, err := s.db.Search(page.Query, searchCap)
		if err != nil {
			slog.Error("searching bodies", "err", err)
			http.Error(w, "search failed", http.StatusInternalServerError)
			return
		}
		page.Searched, page.Total = true, len(hits)
		at := map[string]int{}
		for _, h := range hits {
			i, ok := at[h.Type]
			if !ok {
				i = len(page.Groups)
				at[h.Type] = i
				page.Groups = append(page.Groups, searchGroup{Type: h.Type, Icon: typeIcons[h.Type]})
			}
			page.Groups[i].Hits = append(page.Groups[i].Hits, searchRow{
				Href: artifactHref(h.ID), ID: h.ID, Title: h.Title, Status: h.Status, Glyph: glyphs[h.Status],
				Snippet: markSnippet(h.Snippet),
			})
		}
	}
	s.render(w, r, "search", page)
}

// markSnippet escapes the snippet, then turns the index's \x02/\x03 match
// markers into <mark>. Escaping first keeps body text from injecting markup.
func markSnippet(s string) template.HTML {
	s = template.HTMLEscapeString(s)
	s = strings.ReplaceAll(s, "\x02", "<mark>")
	s = strings.ReplaceAll(s, "\x03", "</mark>")
	return template.HTML(s) //nolint:gosec // escaped above; only our own mark tags are added
}
