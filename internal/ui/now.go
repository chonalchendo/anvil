package ui

import (
	"fmt"
	"net/url"

	"github.com/chonalchendo/anvil/internal/index"
)

// nowCap is how many links a Now column shows before the "N more" link.
const nowCap = 8

// recentLimit is how many recently updated nodes the Now band lists.
const recentLimit = 10

type nowItem struct{ Href, Title, Status, Glyph string }

type nowColumn struct {
	Title    string
	Count    int
	Items    []nowItem
	More     int
	MoreHref string
}

// nowBand is the state-only summary shown above the spine tree.
type nowBand []nowColumn

// nowBand reads the index once per column; it holds no state between requests.
func (s *server) nowBand() (nowBand, error) {
	var ms []index.ArtifactRow
	for _, st := range []string{"in-progress", "planned"} {
		rows, err := s.db.ListByType("milestone", index.QueryFilters{Status: st})
		if err != nil {
			return nil, err
		}
		ms = append(ms, rows...)
	}
	threads, err := s.db.ListByType("thread", index.QueryFilters{Status: "open"})
	if err != nil {
		return nil, err
	}
	drafts, err := s.db.ListByType("learning", index.QueryFilters{Status: "draft"})
	if err != nil {
		return nil, err
	}
	recent, err := s.db.RecentlyUpdated(recentLimit)
	if err != nil {
		return nil, err
	}
	rc := nowColumn{Title: "Recently updated", Count: len(recent)}
	for _, r := range recent {
		rc.Items = append(rc.Items, nowLink(r))
	}
	return nowBand{
		capColumn("Milestones in flight", "milestone", ms),
		capColumn("Open threads", "thread", threads),
		capColumn("Draft learnings", "learning", drafts),
		rc,
	}, nil
}

func nowLink(r index.ArtifactRow) nowItem {
	return nowItem{Href: artifactHref(r.ID), Title: r.Title, Status: r.Status, Glyph: glyphs[r.Status]}
}

// capColumn shows the first nowCap rows; "N more" opens the type list at the first hidden row's status.
func capColumn(title, typ string, rows []index.ArtifactRow) nowColumn {
	c := nowColumn{Title: title, Count: len(rows)}
	for _, r := range rows[:min(len(rows), nowCap)] {
		c.Items = append(c.Items, nowLink(r))
	}
	if len(rows) > nowCap {
		c.More = len(rows) - nowCap
		c.MoreHref = fmt.Sprintf("/type/%s?status=%s", typ, url.QueryEscape(rows[nowCap].Status))
	}
	return c
}
