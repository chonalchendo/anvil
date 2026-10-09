package ui

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sort"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

type typeRow struct {
	Href, ID, Title, Status, Glyph, Updated string
	Tags                                    []tagLink
	Backlinks                               int
}

type tagLink struct{ Name, Href string }

type statusGroup struct {
	Status string
	Rows   []typeRow
}

type statusTab struct {
	Label, Href string
	Count       int
	Current     bool
}

type chip struct{ Label, Remove string }

type projectChip struct {
	Name, Href string
	Current    bool
}

type typePage struct {
	Type, Icon, Project, Status, To, Tag string
	Tabs                                 []statusTab
	Chips                                []chip
	Projects                             []projectChip
	AllHref                              string
	Groups                               []statusGroup
}

// typeList lists one type from the index, grouped by status live-first, newest first within a group.
func (s *server) typeList(w http.ResponseWriter, r *http.Request) {
	t, err := core.ParseType(r.PathValue("type"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	page := typePage{Type: string(t), Icon: typeIcons[string(t)], Project: q.Get("project"), Status: q.Get("status"), To: q.Get("to"), Tag: q.Get("tag")}
	if err := s.fillTypePage(&page); err != nil {
		slog.Error("building type list", "type", t, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "type", page)
}

func (s *server) fillTypePage(page *typePage) error {
	rows, err := s.db.ListByType(page.Type, index.QueryFilters{Project: page.Project})
	if err != nil {
		return err
	}
	tags, err := s.db.TagsByType(page.Type)
	if err != nil {
		return err
	}
	if page.Tag != "" {
		rows = slices.DeleteFunc(rows, func(r index.ArtifactRow) bool { return !slices.Contains(tags[r.ID], page.Tag) })
	}
	if page.To != "" {
		if rows, err = s.citing(rows, page.To); err != nil {
			return err
		}
	}
	page.Tabs = statusTabs(page, rows)
	page.Chips = chips(page)
	if err := s.fillProjects(page); err != nil {
		return err
	}
	if page.Status != "" {
		rows = slices.DeleteFunc(rows, func(r index.ArtifactRow) bool { return r.Status != page.Status })
	}
	back, err := s.db.BacklinkCounts(page.Type)
	if err != nil {
		return err
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
			page.Groups = append(page.Groups, statusGroup{Status: row.Status})
		}
		last := &page.Groups[len(page.Groups)-1]
		tr := typeRow{
			Href: artifactHref(row.ID), ID: row.ID, Title: row.Title, Status: row.Status,
			Glyph: glyphs[row.Status], Updated: row.Updated, Backlinks: back[row.ID],
		}
		for _, tag := range tags[row.ID] {
			tr.Tags = append(tr.Tags, tagLink{Name: tag, Href: typeHref(page, "tag", tag)})
		}
		last.Rows = append(last.Rows, tr)
	}
	return nil
}

// fillProjects lists one chip per project, each keeping the other active filters; an empty project drops the filter.
func (s *server) fillProjects(page *typePage) error {
	projects, err := s.db.Projects()
	if err != nil {
		return err
	}
	for _, p := range projects {
		page.Projects = append(page.Projects, projectChip{Name: p, Href: typeHref(page, "project", p), Current: p == page.Project})
	}
	page.AllHref = typeHref(page, "project", "")
	return nil
}

// typeHref returns the type-list URL for page's filters with key set to val ("" drops it).
func typeHref(page *typePage, key, val string) string {
	v := url.Values{}
	for k, cur := range map[string]string{"project": page.Project, "status": page.Status, "to": page.To, "tag": page.Tag} {
		if k == key {
			cur = val
		}
		if cur != "" {
			v.Set(k, cur)
		}
	}
	if len(v) == 0 {
		return "/type/" + page.Type
	}
	return "/type/" + page.Type + "?" + v.Encode()
}

// statusTabs lists All plus each status present in rows, live-first; rows is already narrowed by every filter but status.
func statusTabs(page *typePage, rows []index.ArtifactRow) []statusTab {
	n := map[string]int{}
	for _, r := range rows {
		n[r.Status]++
	}
	statuses := make([]string, 0, len(n))
	for st := range n {
		statuses = append(statuses, st)
	}
	sort.Slice(statuses, func(a, b int) bool {
		ra, rb := rank(liveOrder, statuses[a]), rank(liveOrder, statuses[b])
		return ra < rb || ra == rb && statuses[a] < statuses[b]
	})
	tabs := []statusTab{{Label: "All", Href: typeHref(page, "status", ""), Count: len(rows), Current: page.Status == ""}}
	if page.Status != "" && n[page.Status] == 0 {
		statuses = append(statuses, page.Status)
	}
	for _, st := range statuses {
		tabs = append(tabs, statusTab{Label: st, Href: typeHref(page, "status", st), Count: n[st], Current: page.Status == st})
	}
	return tabs
}

// chips lists the active tag and link-target filters, each with a link that drops it.
func chips(page *typePage) []chip {
	var out []chip
	for _, c := range []struct{ key, label, val string }{
		{"tag", "tag", page.Tag}, {"to", "cites", page.To},
	} {
		if c.val != "" {
			out = append(out, chip{Label: c.label + " " + c.val, Remove: typeHref(page, c.key, "")})
		}
	}
	return out
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
