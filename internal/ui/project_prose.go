package ui

import (
	"net/url"
	"slices"
	"strconv"

	"github.com/chonalchendo/anvil/internal/index"
)

const (
	headCap     = 2  // titles linked per clause
	acceptedCap = 3  // accepted decisions named
	scanCap     = 40 // newest drafts read for their confidence
)

// clause is one clause of a prose band: a lead, an optional date, linked titles, an optional more-link.
type clause struct {
	Lead, Date, ISO string
	Items           []proseItem
	More, MoreHref  string // a closing "; N more" link
}

type paragraph []clause

// fillLately composes the Decided and Learned bands from the project's decision and learning rows.
func (s *server) fillLately(p *projectPage, counts map[string]map[string]int) error {
	var err error
	if p.Decided, err = s.decidedProse(p.Name, counts); err != nil {
		return err
	}
	p.Learned, err = s.learnedProse(p.Name, counts)
	return err
}

// newestRows returns the project's rows of typ, newest first.
func (s *server) newestRows(typ, project string, counts map[string]map[string]int) ([]index.ArtifactRow, error) {
	if len(counts[typ]) == 0 {
		return nil, nil
	}
	rows, err := s.db.ListByType(typ, index.QueryFilters{Project: project})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(rows, byNewest)
	return rows, nil
}

func withStatus(rows []index.ArtifactRow, status string) []index.ArtifactRow {
	var out []index.ArtifactRow
	for _, r := range rows {
		if r.Status == status {
			out = append(out, r)
		}
	}
	return out
}

// titles links the first n rows. dated puts each row's date after its title.
func titles(rows []index.ArtifactRow, n int, dated bool) []proseItem {
	var out []proseItem
	for _, r := range rows[:min(len(rows), n)] {
		it := proseOf(r)
		if !dated {
			it.Updated, it.UpdatedISO = "", ""
		}
		out = append(out, it)
	}
	joinProse(out)
	return out
}

// decidedProse names the proposals that wait on the human, then the last accepted decisions.
// With more proposals than headCap it names the newest headCap, then the next headCap older
// ones, and links the rest.
func (s *server) decidedProse(project string, counts map[string]map[string]int) ([]paragraph, error) {
	rows, err := s.newestRows("decision", project, counts)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	waiting := withStatus(rows, "proposed")
	out := []paragraph{{{Lead: "No proposal waits on you"}}}
	switch n := len(waiting); {
	case n > 0 && n <= headCap:
		out[0] = paragraph{{Lead: plural(n, "proposal", "proposals") + " " + pluralWord(n, "waits", "wait") + " on you:", Items: titles(waiting, headCap, true)}}
	case n > headCap:
		older := waiting[headCap:]
		c := clause{Lead: "Older:", Items: titles(older, headCap, true)}
		if more := len(older) - headCap; more > 0 {
			c.More, c.MoreHref = strconv.Itoa(more)+" more", "/type/decision?project="+url.QueryEscape(project)+"&status=proposed"
		}
		out[0] = paragraph{
			{Lead: plural(n, "proposal", "proposals") + " wait on you"},
			{Lead: "Newest:", Items: titles(waiting, headCap, true)},
			c,
		}
	}
	if accepted := withStatus(rows, "accepted"); len(accepted) > 0 {
		out = append(out, paragraph{{Lead: "Accepted last:", Items: titles(accepted, acceptedCap, true)}})
	}
	return out, nil
}

// learnedProse counts drafts and verified learnings, then names the newest drafts, the
// high-confidence drafts that wait for a check, and the newest verified ones.
func (s *server) learnedProse(project string, counts map[string]map[string]int) ([]paragraph, error) {
	rows, err := s.newestRows("learning", project, counts)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	drafts, verified := withStatus(rows, "draft"), withStatus(rows, "verified")
	verifiedText := "none verified"
	if len(verified) > 0 {
		verifiedText = strconv.Itoa(len(verified)) + " verified"
	}
	head := clause{
		Lead: plural(len(drafts), "draft", "drafts") + " and " + verifiedText + "; nothing new since",
		Date: shortDate(rows[0].Updated), ISO: day(rows[0].Updated),
	}
	out := []paragraph{{head}}
	var newest, held []index.ArtifactRow
	conf := map[string]string{}
	for _, r := range drafts[:min(len(drafts), scanCap)] {
		if len(newest) == headCap && len(held) == headCap {
			break
		}
		_, art, err := s.load(r.ID)
		if err != nil {
			return nil, err
		}
		c, _ := art.FrontMatter["confidence"].(string)
		switch {
		case len(newest) < headCap:
			newest = append(newest, r)
			conf[r.ID] = c
		case c == "high" && len(held) < headCap:
			held = append(held, r)
		}
	}
	var second paragraph
	if len(newest) > 0 {
		items := titles(newest, headCap, false)
		lead := "Newest drafts:"
		if shared := conf[newest[0].ID]; shared != "" && sameConfidence(newest, conf) {
			lead = "Newest, " + pluralWord(len(newest), "", "both ") + "at " + shared + " confidence:"
		} else {
			for i := range items {
				if c := conf[newest[i].ID]; c != "" {
					items[i].Note = "at " + c + " confidence"
				}
			}
		}
		out[0] = append(out[0], clause{Lead: lead, Items: items})
	}
	if len(held) > 0 {
		second = append(second, clause{Lead: "Held at high confidence, unverified:", Items: titles(held, headCap, false)})
	}
	if len(verified) > 0 {
		second = append(second, clause{Lead: "Verified:", Items: titles(verified, headCap, false)})
	}
	if len(second) > 0 {
		out = append(out, second)
	}
	return out, nil
}

func sameConfidence(rows []index.ArtifactRow, conf map[string]string) bool {
	for _, r := range rows {
		if conf[r.ID] != conf[rows[0].ID] {
			return false
		}
	}
	return true
}
