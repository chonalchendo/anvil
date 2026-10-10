package ui

import (
	"regexp"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/index"
)

// doneCap is how many done milestones the Recently done band lists.
const doneCap = 6

type doneRow struct {
	proseItem
	Resolved, Total int
	SHA             string
}

type designs struct {
	Canvases []canvas
	Product  []node
	System   []node
	Comps    []node
}

// fillDone fills the newest done milestones.
func (s *server) fillDone(p *projectPage, counts map[string]map[string]int, members map[string][]index.ArtifactRow) error {
	if counts["milestone"]["done"] == 0 {
		return nil
	}
	ms, err := s.db.ListByType("milestone", index.QueryFilters{Project: p.Name, Status: "done"})
	if err != nil {
		return err
	}
	slices.SortStableFunc(ms, byNewest)
	ms = ms[:min(len(ms), doneCap)]
	for _, r := range ms {
		_, art, err := s.load(r.ID)
		if err != nil {
			return err
		}
		var resolved, total int
		for _, i := range members[r.ID] {
			if i.Status == "resolved" {
				resolved++
			}
			if i.Status != "abandoned" {
				total++
			}
		}
		p.Done = append(p.Done, doneRow{proseOf(r), resolved, total, measuredSHA(art.Body)})
	}
	return nil
}

// fillDesigns fills the designs strip: every diagram of the project's product and system designs, then the spine links.
func (s *server) fillDesigns(p *projectPage, counts map[string]map[string]int) error {
	for _, d := range []struct {
		typ      string
		dst      *[]node
		diagrams bool
	}{
		{"product-design", &p.Designs.Product, true},
		{"system-design", &p.Designs.System, true},
		{"component-design", &p.Designs.Comps, false},
	} {
		if len(counts[d.typ]) == 0 {
			continue
		}
		rows, err := s.db.ListByType(d.typ, index.QueryFilters{Project: p.Name})
		if err != nil {
			return err
		}
		for _, r := range rows {
			*d.dst = append(*d.dst, leaf(r))
			if !d.diagrams {
				continue
			}
			_, art, err := s.load(r.ID)
			if err != nil {
				return err
			}
			if d.typ == "product-design" && p.Deck == "" {
				p.Deck, _ = art.FrontMatter["description"].(string)
			}
			for _, c := range diagramsOf(art.FrontMatter) {
				if !slices.ContainsFunc(p.Designs.Canvases, func(have canvas) bool { return have.Name == c.Name }) {
					c.Note, c.Lazy = diagramNote(art.Body, c.Name), true
					p.Designs.Canvases = append(p.Designs.Canvases, c)
				}
			}
		}
	}
	return nil
}

// diagramNote returns the last sentence of the paragraph above a design's
// "Diagram `name`" line, or "" when the body has none.
func diagramNote(body, name string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if !strings.Contains(l, "Diagram `"+name+"`") {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			t := strings.TrimSpace(lines[j])
			if t == "" {
				continue
			}
			if strings.HasPrefix(t, "#") {
				return ""
			}
			t = strings.NewReplacer("`", "", "*", "").Replace(t)
			sentences := strings.SplitAfter(t, ". ")
			note := strings.TrimSpace(sentences[len(sentences)-1])
			if r := []rune(note); len(r) > 100 {
				note = string(r[:99]) + "…"
			}
			return note
		}
	}
	return ""
}

var shaRe = regexp.MustCompile(`\bat ([0-9a-f]{7,40})\b`)

// measuredSHA returns the commit in a milestone's last Measured line, or "".
func measuredSHA(body string) string {
	if m := shaRe.FindStringSubmatch(lastMeasured(body)); m != nil {
		return m[1]
	}
	return ""
}

// joinProse sets each item's Sep so the items read as "a, b and c".
func joinProse(items []proseItem) {
	for i := range items {
		switch {
		case i == 0:
		case i == len(items)-1:
			items[i].Sep = " and "
		default:
			items[i].Sep = ", "
		}
	}
}
