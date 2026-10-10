package ui

import (
	"fmt"
	"strings"

	"github.com/chonalchendo/anvil/internal/index"
)

// milestoneMembers maps a milestone id to the issues whose milestone slot names it, in full or
// as the bare slug without the project prefix. It owns membership for the dashboard folds, the
// lead counts, and Recently done. An issue with no slot, or a slot naming no
// listed milestone, is in no entry.
func (s *server) milestoneMembers(project string, issues, milestones []index.ArtifactRow) (map[string][]index.ArtifactRow, error) {
	known := map[string]bool{}
	for _, m := range milestones {
		known[m.ID] = true
	}
	out := map[string][]index.ArtifactRow{}
	for _, i := range issues {
		links, err := s.db.LinksFrom(i.ID)
		if err != nil {
			return nil, fmt.Errorf("links from %s: %w", i.ID, err)
		}
		id := slotOf(links, "milestone")
		if !known[id] {
			id = milestoneKey(project, id)
		}
		if known[id] {
			out[id] = append(out[id], i)
		}
	}
	return out, nil
}

// milestoneKey expands a milestone slot with no project segment (`slug` or `milestone.slug`)
// to its index key. A slot that already carries a project is returned as is, so a dangling
// full key stays dangling. It is the one owner of the bare-slug rule.
func milestoneKey(project, slot string) string {
	slug := strings.TrimPrefix(slot, "milestone.")
	if strings.Contains(slug, ".") {
		return slot
	}
	return "milestone." + project + "." + slug
}
