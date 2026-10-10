package ui

import (
	"slices"
	"strconv"

	"github.com/chonalchendo/anvil/internal/index"
)

const (
	headCap     = 2  // titles linked per clause
	acceptedCap = 3  // accepted decisions named
	scanCap     = 40 // newest drafts read for their confidence
)

// clause is one clause of a prose band: a lead, an optional date, then linked titles.
type clause struct {
	Lead, Date, ISO string
	Items           []proseItem
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
func (s *server) decidedProse(project string, counts map[string]map[string]int) ([]paragraph, error) {
	rows, err := s.newestRows("decision", project, counts)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	waiting := withStatus(rows, "proposed")
	out := []paragraph{{{Lead: "No proposal waits on you"}}}
	if n := len(waiting); n > 0 {
		out[0] = paragraph{{Lead: plural(n, "proposal", "proposals") + " " + pluralWord(n, "waits", "wait") + " on you:", Items: titles(waiting, headCap, true)}}
		if rest := waiting[min(n, headCap):]; len(rest) > 0 {
			lead := plural(len(rest), "older one has", "older ones have") + " waited:"
			if len(rest) > headCap {
				lead = plural(len(rest), "older one has", "older ones have") + " waited, the newest " + strconv.Itoa(headCap) + ":"
			}
			out[0] = append(out[0], clause{Lead: lead, Items: titles(rest, headCap, true)})
		}
	}
	if accepted := withStatus(rows, "accepted"); len(accepted) > 0 {
		out = append(out, paragraph{{Lead: "Accepted last:", Items: titles(accepted, acceptedCap, true)}})
	}
	return out, nil
}

// learnedProse counts drafts and verified learnings, then names the newest drafts at their
// confidence, the high-confidence drafts that wait for a check, and the newest verified ones.
func (s *server) learnedProse(project string, counts map[string]map[string]int) ([]paragraph, error) {
	rows, err := s.newestRows("learning", project, counts)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	drafts, verified := withStatus(rows, "draft"), withStatus(rows, "verified")
	head := clause{
		Lead: plural(len(drafts), "draft", "drafts") + " and " + strconv.Itoa(len(verified)) + " verified, last updated",
		Date: shortDate(rows[0].Updated), ISO: day(rows[0].Updated),
	}
	out := []paragraph{{head}}
	var newest, held []index.ArtifactRow
	notes := map[string]string{}
	for _, r := range drafts[:min(len(drafts), scanCap)] {
		_, art, err := s.load(r.ID)
		if err != nil {
			return nil, err
		}
		conf, _ := art.FrontMatter["confidence"].(string)
		if len(newest) < headCap {
			newest = append(newest, r)
			notes[r.ID] = "at " + conf + " confidence"
		}
		if conf == "high" && len(held) < headCap {
			held = append(held, r)
		}
	}
	var second paragraph
	if len(newest) > 0 {
		items := titles(newest, headCap, false)
		for i := range items {
			items[i].Note = notes[newest[i].ID]
		}
		second = append(second, clause{Lead: "Newest drafts:", Items: items})
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
