package cli

import (
	"sort"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// readyUnit is a ready issue enriched with the start-context an agent needs to
// begin it. It is the deterministic selection unit shared by `anvil next`
// (which returns the head) and `anvil build` (which dispatches the full ordered
// frontier) — per contract.anvil.build-orchestration-contract, work-selection
// lives in the driver, not the engine. `created` is lower-case because it feeds
// the sort only; it is not part of the start-context an agent consumes.
type readyUnit struct {
	ID        string   `json:"id"`
	Goal      string   `json:"goal"`
	Severity  string   `json:"severity"`
	Milestone string   `json:"milestone"`
	Contracts []string `json:"contracts"`
	Path      string   `json:"path"`
	created   string
}

// severityRank orders the issue severity enum (low|medium|high|critical) for the
// priority sort. Unknown or empty severities rank lowest so a malformed issue
// never jumps the queue.
var severityRank = map[string]int{
	"critical": 4,
	"high":     3,
	"medium":   2,
	"low":      1,
}

// selectReadyUnits enriches the ready-issue frontier with start-context and
// returns it in deterministic priority order: severity desc, then created asc,
// then id asc. A stable total order means repeated calls and parallel agents
// agree on the same next unit. When milestone is non-empty, rows whose milestone
// frontmatter does not match are dropped. A row whose body cannot be loaded is
// skipped — a ready issue with an unreadable body cannot be dispatched.
func selectReadyUnits(rows []index.ArtifactRow, milestone string) []readyUnit {
	units := make([]readyUnit, 0, len(rows))
	for _, r := range rows {
		a, err := core.LoadArtifact(r.Path)
		if err != nil {
			continue
		}
		ms := milestoneSlug(a.FrontMatter["milestone"])
		if milestone != "" && ms != milestone {
			continue
		}
		goal, _ := a.FrontMatter["goal"].(string)
		sev, _ := a.FrontMatter["severity"].(string)
		units = append(units, readyUnit{
			ID:        r.ID,
			Goal:      goal,
			Severity:  sev,
			Milestone: ms,
			Contracts: contractIDs(a.FrontMatter["related"]),
			Path:      r.Path,
			created:   r.Created,
		})
	}
	sort.SliceStable(units, func(i, j int) bool {
		if ri, rj := severityRank[units[i].Severity], severityRank[units[j].Severity]; ri != rj {
			return ri > rj
		}
		if units[i].created != units[j].created {
			return units[i].created < units[j].created
		}
		return units[i].ID < units[j].ID
	})
	return units
}

// contractIDs extracts contract ids from an issue's `related` wikilink array:
// `[[contract.<id>]]` → `<id>`. Non-contract relations (milestone spine, sibling
// issues) are ignored — only governing contracts belong in start-context.
func contractIDs(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	const prefix = "[[contract."
	const suffix = "]]"
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, prefix) && strings.HasSuffix(s, suffix) {
			out = append(out, s[len(prefix):len(s)-len(suffix)])
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
