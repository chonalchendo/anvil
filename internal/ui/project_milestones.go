package ui

import (
	"fmt"
	"html/template"
	"regexp"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/index"
)

// issueRow is one not-done issue in a milestone fold. Empty fields render as a dash.
type issueRow struct {
	Ord, Verdict, PR, PRHref, Rounds, Tokens, Updated, UpdatedISO, Owner string
	node
}

type statusN struct {
	Status, Label, Sep string
	N                  int
}

// msFold is one milestone with its issues, or the "No milestone" group.
type msFold struct {
	node
	Open               bool
	Approved, Measured string
	Resolved, Total    int
	Abandoned          int
	NotDone            []statusN
	Issues             []issueRow
	Live               bool
	activity, stamp    string
}

// milestonesPanel is the dashboard's lead panel: not-done milestones, live work first.
// Nothing is set when no issue or milestone is in progress; the template then writes the empty-state inset.
type milestonesPanel struct {
	Lead    string
	Folds   []msFold
	Nothing bool
	Inset   template.HTML
}

var ordRe = regexp.MustCompile(`\.(\d+)[.-]`)

// fillMilestones builds the panel from one milestone read, one issue read and a frontmatter
// read of each not-done issue.
func (s *server) fillMilestones(p *projectPage, ms, issues []index.ArtifactRow, members map[string][]index.ArtifactRow, product []node) error {
	placed := map[string]bool{}
	var folds []msFold
	for _, m := range ms {
		if m.Status != "in-progress" && m.Status != "planned" {
			continue
		}
		f, err := s.newFold(m, members[m.ID])
		if err != nil {
			return err
		}
		for _, i := range members[m.ID] {
			placed[i.ID] = true
		}
		folds = append(folds, f)
	}
	slices.SortStableFunc(folds, func(a, b msFold) int {
		if d := foldTier(a) - foldTier(b); d != 0 {
			return d
		}
		return strings.Compare(b.activity, a.activity)
	})
	none, err := s.unplaced(issues, placed)
	if err != nil {
		return err
	}
	p.Milestones = milestonePanelOf(folds, none, product)
	return nil
}

func foldTier(f msFold) int {
	switch {
	case f.Live:
		return 0
	case f.Status == "in-progress":
		return 1
	}
	return 2
}

func (s *server) newFold(m index.ArtifactRow, members []index.ArtifactRow) (msFold, error) {
	_, art, err := s.load(m.ID)
	if err != nil {
		return msFold{}, err
	}
	f := msFold{node: leaf(m), Measured: measuredDate(art.Body)}
	f.Approved = shortDate(fmString(art.FrontMatter["approved"]))
	counts := map[string]int{}
	var open []index.ArtifactRow
	for _, i := range members {
		switch i.Status {
		case "resolved":
			f.Resolved++
			f.Total++
		case "abandoned":
			f.Abandoned++
		default:
			f.Total++
			f.activity = max(f.activity, i.Updated)
			counts[i.Status]++
			open = append(open, i)
		}
	}
	f.Live = counts["in-progress"] > 0
	f.stamp = newestStamp(open)
	f.Open = f.Live
	f.NotDone = notDone(counts)
	f.Issues, err = s.issueRows(open)
	return f, err
}

// notDone orders the per-status counts live-first.
func notDone(counts map[string]int) []statusN {
	var out []statusN
	for st, n := range counts {
		out = append(out, statusN{Status: st, Label: strings.ReplaceAll(st, "-", " "), N: n})
	}
	slices.SortFunc(out, func(a, b statusN) int { return rank(liveOrder, a.Status) - rank(liveOrder, b.Status) })
	for i := range out {
		if i > 0 {
			out[i].Sep = ", "
		}
	}
	return out
}

// issueRows reads each issue's frontmatter, newest update first.
func (s *server) issueRows(rows []index.ArtifactRow) ([]issueRow, error) {
	slices.SortStableFunc(rows, byNewest)
	out := make([]issueRow, 0, len(rows))
	for _, r := range rows {
		_, art, err := s.load(r.ID)
		if err != nil {
			return nil, err
		}
		fm := art.FrontMatter
		row := issueRow{
			node:       leaf(r),
			Verdict:    fmString(fm["verified_verdict"]),
			Rounds:     count(fm["cost_rounds"]),
			Tokens:     tokens(fm["cost_tokens"]),
			Updated:    shortDate(r.Updated),
			UpdatedISO: r.Updated[:min(len(r.Updated), 10)],
			Owner:      fmString(fm["owner"]),
		}
		if m := ordRe.FindStringSubmatch(r.ID); m != nil {
			row.Ord = m[1]
		}
		links, _ := fm["external_links"].([]any)
		for _, l := range links {
			if u, _ := l.(string); u != "" {
				if n, ok := prNumber(u); ok {
					row.PRHref, row.PR = u, "#"+n
					break
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// milestoneIssues lists every issue of a milestone, newest first, for its page. in is the
// page's incoming links; the bare-slug links cost one more read.
func (s *view) milestoneIssues(key, project string, in []index.LinkRow) ([]issueRow, error) {
	bare, err := s.db.LinksToAny(milestoneSlots(project, key))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var rows []index.ArtifactRow
	for _, l := range append(in, bare...) {
		r, ok := s.res.rows[l.Source]
		if l.Relation != "milestone" || !ok || r.Type != "issue" || r.Project != project || seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		rows = append(rows, r)
	}
	return s.issueRows(rows)
}

// unplaced returns the not-done issues under no not-done milestone, as the last fold: under
// no milestone, or under a done one.
func (s *server) unplaced(issues []index.ArtifactRow, placed map[string]bool) (*msFold, error) {
	var rows []index.ArtifactRow
	for _, i := range issues {
		if i.Status != "resolved" && i.Status != "abandoned" && !placed[i.ID] {
			rows = append(rows, i)
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	f := &msFold{node: node{Title: "No open milestone"}}
	counts := map[string]int{}
	for _, i := range rows {
		counts[i.Status]++
	}
	f.NotDone = notDone(counts)
	f.stamp = newestStamp(rows)
	var err error
	f.Issues, err = s.issueRows(rows)
	return f, err
}

// measuredDate returns the date of a milestone's last Measured line as a short date, or "".
func measuredDate(body string) string {
	date, _, _ := strings.Cut(lastMeasured(body), ",")
	if d := shortDate(strings.TrimSpace(date)); d != strings.TrimSpace(date) {
		return d
	}
	return ""
}

func fmString(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func count(v any) string {
	if n, ok := num(v); ok {
		return fmt.Sprint(int64(n))
	}
	return ""
}

// tokens writes a token count the short way: 840, 12.4k, 23.3M.
func tokens(v any) string {
	n, ok := num(v)
	switch {
	case !ok:
		return ""
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", n/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fk", n/1e3)
	}
	return fmt.Sprint(int64(n))
}

// newestStamp returns the latest updated date among the in-progress rows, or "".
func newestStamp(rows []index.ArtifactRow) string {
	var got string
	for _, r := range rows {
		if r.Status == "in-progress" {
			got = max(got, r.Updated)
		}
	}
	return got
}

// statusCount returns how many issues hold status.
func statusCount(ns []statusN, status string) int {
	for _, n := range ns {
		if n.Status == status {
			return n.N
		}
	}
	return 0
}
