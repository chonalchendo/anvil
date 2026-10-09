package ui

import (
	"log/slog"
	"net/http"
	"slices"
	"sort"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

type typeRow struct {
	Href, Title, Project, Updated string
}

type statusGroup struct {
	Status, Glyph string
	Rows          []typeRow
}

type typePage struct {
	Type, Project, Status, To string
	Groups                    []statusGroup
}

// typeList lists one type from the index, grouped by status live-first, newest first within a group.
func (s *server) typeList(w http.ResponseWriter, r *http.Request) {
	t, err := core.ParseType(r.PathValue("type"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	page := typePage{Type: string(t), Project: q.Get("project"), Status: q.Get("status"), To: q.Get("to")}
	rows, err := s.db.ListByType(page.Type, index.QueryFilters{Project: page.Project, Status: page.Status})
	if err != nil {
		slog.Error("listing type", "type", t, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	if page.To != "" {
		if rows, err = s.citing(rows, page.To); err != nil {
			slog.Error("filtering by link target", "to", page.To, "err", err)
			http.Error(w, "page failed", http.StatusInternalServerError)
			return
		}
	}
	sort.SliceStable(rows, func(a, b int) bool {
		ra, rb := rank(liveOrder, rows[a].Status), rank(liveOrder, rows[b].Status)
		if ra != rb {
			return ra < rb
		}
		if rows[a].Status != rows[b].Status {
			return rows[a].Status < rows[b].Status
		}
		return rows[a].Updated > rows[b].Updated
	})
	for _, row := range rows {
		if n := len(page.Groups); n == 0 || page.Groups[n-1].Status != row.Status {
			page.Groups = append(page.Groups, statusGroup{Status: row.Status, Glyph: glyphs[row.Status]})
		}
		last := &page.Groups[len(page.Groups)-1]
		last.Rows = append(last.Rows, typeRow{
			Href: artifactHref(row.ID), Title: row.Title, Project: row.Project,
			Updated: row.Updated,
		})
	}
	s.render(w, r, "type", page)
}

// citing keeps the rows that link to target.
func (s *server) citing(rows []index.ArtifactRow, target string) ([]index.ArtifactRow, error) {
	in, err := s.db.LinksTo(target)
	if err != nil {
		return nil, err
	}
	from := make(map[string]bool, len(in))
	for _, l := range in {
		from[l.Source] = true
	}
	return slices.DeleteFunc(rows, func(r index.ArtifactRow) bool { return !from[r.ID] }), nil
}
