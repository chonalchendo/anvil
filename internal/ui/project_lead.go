package ui

import (
	"fmt"
	"html"
	"html/template"
	"slices"
	"strings"
	"time"
)

// openCounts is the open-issue tally behind the lead sentence and the empty-state inset.
type openCounts struct {
	// Under is open issues in the not-done milestones; None is open issues in the last fold.
	Under, None, Planned, Folds int
	Most                        *node
}

func (c openCounts) all() int { return c.Under + c.None }

func countOpen(folds []msFold, none *msFold) openCounts {
	c := openCounts{Folds: len(folds)}
	var most int
	for i, f := range folds {
		c.Under += statusCount(f.NotDone, "open")
		if f.Status != "in-progress" {
			c.Planned++
		}
		if n := sumStatus(f.NotDone); n > most {
			most, c.Most = n, &folds[i].node
		}
	}
	if none != nil {
		c.None = statusCount(none.NotDone, "open")
	}
	return c
}

// milestonePanelOf builds the panel and its lead sentence from the sorted folds and the last fold.
func milestonePanelOf(folds []msFold, none *msFold) milestonesPanel {
	pn := milestonesPanel{Folds: folds}
	byStatus := map[string]int{}
	var live, bare, ip int
	var stamp string
	for _, f := range folds {
		byStatus[f.Status]++
		if f.Status == "in-progress" && !f.Live {
			bare++
		}
		if f.Live {
			live++
		}
	}
	all := slices.Clone(folds)
	if none != nil {
		all = append(all, *none)
		pn.Folds = append(pn.Folds, *none)
	}
	for _, f := range all {
		if n := statusCount(f.NotDone, "in-progress"); n > 0 {
			ip += n
			stamp = max(stamp, f.stamp)
		}
	}
	pn.Nothing = ip == 0 && bare == 0 && live == 0
	oc := countOpen(folds, none)

	var b strings.Builder
	if len(folds) == 0 {
		b.WriteString("No milestone is planned or in progress.")
	} else {
		var parts []string
		for _, st := range milestoneOrder {
			if n := byStatus[st]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", n, strings.ReplaceAll(st, "-", " ")))
			}
		}
		fmt.Fprintf(&b, "%s not done: %s.", plural(len(folds), "milestone is", "milestones are"), strings.Join(parts, " and "))
	}
	if ip > 0 {
		switch {
		case live == 1:
			b.WriteString(" The milestone holding live work opens first:")
		case live > 1:
			fmt.Fprintf(&b, " The %d holding live work open first:", live)
		}
		fmt.Fprintf(&b, " %s, last updated %s%s.", plural(ip, "issue is in progress", "issues are in progress"), shortDate(stamp), ago(stamp))
	}
	if len(folds) > 0 && bare == 0 {
		b.WriteString(" No milestone is in progress without live work.")
	}
	if oc.all() > 0 {
		b.WriteString(" " + openWhere(oc) + ".")
	}
	pn.Lead = strings.TrimSpace(b.String())
	return pn
}

// openWhere writes where the open issues sit, e.g. "11 open issues sit under the 3 milestones,
// and 2 under none or a done milestone".
func openWhere(c openCounts) string {
	const none = "under none or a done milestone"
	switch {
	case c.None == 0:
		return fmt.Sprintf("%s under the %s", plural(c.Under, "open issue sits", "open issues sit"), plural(c.Folds, "milestone", "milestones"))
	case c.Under == 0:
		return fmt.Sprintf("%s %s", plural(c.None, "open issue sits", "open issues sit"), none)
	}
	return fmt.Sprintf("%s under the %s, and %d %s", plural(c.Under, "open issue sits", "open issues sit"), plural(c.Folds, "milestone", "milestones"), c.None, none)
}

// insetOf writes the empty-state inset shown when no work is in progress. The titles and
// hrefs are escaped; the markup is built here so the plural and the clauses stay in one place.
func insetOf(pn milestonesPanel, folds []msFold, none *msFold, product []node) template.HTML {
	if !pn.Nothing {
		return ""
	}
	oc := countOpen(folds, none)
	if oc.Planned == 0 && oc.all() == 0 {
		out := "No milestone is in flight or planned."
		if len(product) > 0 {
			out += fmt.Sprintf(` The product design lists the next candidates under <a href="%s#milestones">Milestones</a>.`, html.EscapeString(product[0].Href))
		}
		return template.HTML(out) //nolint:gosec // every interpolated value is escaped above
	}
	out := "Nothing is in progress."
	if oc.all() == 0 {
		out += " No issue is open"
	} else {
		var where []string
		if oc.Under > 0 {
			where = append(where, fmt.Sprintf("%d under the %s", oc.Under, plural(oc.Planned, "planned milestone", "planned milestones")))
		}
		if oc.None > 0 {
			where = append(where, fmt.Sprintf("%d under none or a done milestone", oc.None))
		}
		out += fmt.Sprintf(" %s open: %s", plural(oc.all(), "issue is", "issues are"), strings.Join(where, " and "))
	}
	if oc.Most != nil {
		out += fmt.Sprintf(`; <a href="%s" class="to-%s">%s</a> holds the most`, html.EscapeString(oc.Most.Href), html.EscapeString(oc.Most.Status), html.EscapeString(oc.Most.Title))
	}
	return template.HTML(out + ".") //nolint:gosec // every interpolated value is escaped above
}

func sumStatus(ns []statusN) int {
	t := 0
	for _, n := range ns {
		t += n.N
	}
	return t
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// ago writes ", N days ago" for a date at least a day old, else "".
func ago(date string) string {
	if len(date) < 10 {
		return ""
	}
	t, err := time.Parse(time.DateOnly, date[:10])
	if err != nil {
		return ""
	}
	if d := int(time.Since(t).Hours() / 24); d >= 1 {
		return ", " + plural(d, "day", "days") + " ago"
	}
	return ""
}
