package ui

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/index"
)

// recentLimit is how many recently updated nodes the Now band lists.
const recentLimit = 10

type nowColumn struct {
	Title    string
	Count    int
	Items    []node
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
	rc := nowColumn{Title: "Recently updated"}
	for _, r := range recent {
		rc.Items = append(rc.Items, leaf(r))
	}
	return nowBand{
		capColumn("Milestones in flight", "milestone", ms),
		capColumn("Open threads", "thread", threads),
		capColumn("Draft learnings", "learning", drafts),
		rc,
	}, nil
}

// byNewest orders rows by updated date, newest first.
func byNewest(a, b index.ArtifactRow) int { return strings.Compare(b.Updated, a.Updated) }

// capColumn shows the newest treeCap rows. "N more" opens the type list, filtered to the hidden rows' status only when they share one.
func capColumn(title, typ string, rows []index.ArtifactRow) nowColumn {
	slices.SortStableFunc(rows, byNewest)
	c := nowColumn{Title: title, Count: len(rows)}
	for _, r := range rows[:min(len(rows), treeCap)] {
		c.Items = append(c.Items, leaf(r))
	}
	if len(rows) > treeCap {
		hidden := rows[treeCap:]
		c.More = len(hidden)
		c.MoreHref = "/type/" + typ
		if !slices.ContainsFunc(hidden, func(r index.ArtifactRow) bool { return r.Status != hidden[0].Status }) {
			c.MoreHref = fmt.Sprintf("%s?status=%s", c.MoreHref, url.QueryEscape(hidden[0].Status))
		}
	}
	return c
}
