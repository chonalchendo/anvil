package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/index"
)

// issueRow is one not-done issue in a milestone fold. Empty fields render as a dash.
type issueRow struct {
	Ord, Verdict, PR, PRHref, Rounds, Tokens, Updated, Owner string
	node
}

type statusN struct {
	Status, Sep string
	N           int
}

// msFold is one milestone with its issues, or the "No milestone" group.
type msFold struct {
	node
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
	Lead                                  string
	Folds                                 []msFold
	Nothing                               bool
	OpenAll, OpenUnder, OpenNone, Planned int
	Most                                  *node
}

var ordRe = regexp.MustCompile(`\.(\d+)[.-]`)

// fillMilestones builds the panel from one milestone read, one issue read and a frontmatter
// read of each not-done issue.
func (s *server) fillMilestones(p *projectPage, _ map[string]map[string]int) error {
	ms, err := s.db.ListByType("milestone", index.QueryFilters{Project: p.Name})
	if err != nil {
		return err
	}
	issues, err := s.db.ListByType("issue", index.QueryFilters{Project: p.Name})
	if err != nil {
		return err
	}
	byID := map[string]index.ArtifactRow{}
	for _, i := range issues {
		byID[i.ID] = i
	}
	known := map[string]bool{}
	for _, m := range ms {
		known[m.ID] = true
	}
	placed := map[string]bool{}
	var folds []msFold
	for _, m := range ms {
		if m.Status != "in-progress" && m.Status != "planned" {
			continue
		}
		members, err := s.memberIssues(m.ID, p.Name, byID)
		if err != nil {
			return err
		}
		f, err := s.newFold(m, members)
		if err != nil {
			return err
		}
		for _, i := range members {
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
	none, err := s.unplaced(issues, placed, known, p.Name)
	if err != nil {
		return err
	}
	p.Milestones = milestonePanelOf(folds, none)
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

// memberIssues returns the issues whose milestone slot names ms, in full or as the bare slug
// without the project prefix.
func (s *server) memberIssues(ms, project string, byID map[string]index.ArtifactRow) ([]index.ArtifactRow, error) {
	var out []index.ArtifactRow
	seen := map[string]bool{}
	for _, target := range []string{ms, "milestone." + strings.TrimPrefix(ms, "milestone."+project+".")} {
		in, err := s.db.LinksTo(target)
		if err != nil {
			return nil, fmt.Errorf("links to %s: %w", target, err)
		}
		for _, l := range in {
			if i, ok := byID[l.Source]; ok && l.Relation == "milestone" && !seen[i.ID] {
				seen[i.ID] = true
				out = append(out, i)
			}
		}
	}
	return out, nil
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
		f.activity = max(f.activity, i.Updated)
		switch i.Status {
		case "resolved":
			f.Resolved++
			f.Total++
		case "abandoned":
			f.Abandoned++
		default:
			f.Total++
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
		out = append(out, statusN{Status: st, N: n})
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
			node:    leaf(r),
			Verdict: fmString(fm["verified_verdict"]),
			Rounds:  count(fm["cost_rounds"]),
			Tokens:  tokens(fm["cost_tokens"]),
			Updated: shortDate(r.Updated),
			Owner:   fmString(fm["owner"]),
		}
		if m := ordRe.FindStringSubmatch(r.ID); m != nil {
			row.Ord = m[1]
		}
		links, _ := fm["external_links"].([]any)
		for _, l := range links {
			if u, _ := l.(string); strings.Contains(u, "/pull/") {
				row.PRHref, row.PR = u, "#"+u[strings.LastIndex(u, "/")+1:]
				break
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// unplaced returns the not-done issues under no milestone, as the "No milestone" fold.
func (s *server) unplaced(issues []index.ArtifactRow, placed, known map[string]bool, project string) (*msFold, error) {
	var rows []index.ArtifactRow
	for _, i := range issues {
		if i.Status == "resolved" || i.Status == "abandoned" || placed[i.ID] {
			continue
		}
		_, art, err := s.load(i.ID)
		if err != nil {
			return nil, err
		}
		slot := strings.Trim(fmString(art.FrontMatter["milestone"]), "[]")
		if known[slot] || known["milestone."+project+"."+strings.TrimPrefix(slot, "milestone.")] {
			continue
		}
		rows = append(rows, i)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	f := &msFold{node: node{Title: "No milestone"}}
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

// milestonePanelOf builds the panel and its lead sentence from the sorted folds and the "No milestone" fold.
func milestonePanelOf(folds []msFold, none *msFold) milestonesPanel {
	pn := milestonesPanel{Folds: folds, Nothing: true}
	byStatus := map[string]int{}
	var live, bare, ip, most int
	var stamp string
	for i, f := range folds {
		byStatus[f.Status]++
		pn.OpenUnder += statusCount(f.NotDone, "open")
		if f.Status != "in-progress" {
			pn.Planned++
		} else if !f.Live {
			bare++
		}
		if f.Live {
			live++
		}
		if n := sumStatus(f.NotDone); n > most {
			most, pn.Most = n, &folds[i].node
		}
	}
	for _, f := range append(slices.Clone(folds), noneOrEmpty(none)) {
		if n := statusCount(f.NotDone, "in-progress"); n > 0 {
			ip += n
			stamp = max(stamp, f.stamp)
		}
	}
	if none != nil {
		pn.OpenNone = statusCount(none.NotDone, "open")
		pn.Folds = append(pn.Folds, *none)
	}
	pn.OpenAll = pn.OpenUnder + pn.OpenNone
	pn.Nothing = ip == 0 && bare == 0 && live == 0

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
	if len(folds) > 0 {
		if bare == 0 {
			b.WriteString(" No milestone is in progress without live work.")
		}
		fmt.Fprintf(&b, " %d open issues sit under the %d, and %d under none.", pn.OpenUnder, len(folds), pn.OpenNone)
	}
	pn.Lead = strings.TrimSpace(b.String())
	return pn
}

func noneOrEmpty(f *msFold) msFold {
	if f == nil {
		return msFold{}
	}
	return *f
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
