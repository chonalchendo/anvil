package cli

import (
	"fmt"
	"sort"

	"github.com/chonalchendo/anvil/internal/core"
)

// issueCost holds the four cost_* fields an issue carries once landed.
type issueCost struct {
	Rounds int `json:"rounds"`
	Diff   int `json:"diff"`
	Files  int `json:"files"`
	Tokens int `json:"tokens"`
}

// milestoneIssueRow is one issue of a milestone; Cost is nil unless all four
// cost fields are present.
type milestoneIssueRow struct {
	ID     string     `json:"id"`
	Status string     `json:"status"`
	Cost   *issueCost `json:"cost"`
}

type costTotal struct {
	issueCost
	Costed int `json:"costed"`
	Issues int `json:"issues"`
}

// milestoneCostRows returns the issues linked to ms (bare slug) sorted by id,
// and the sum of their costed rows. A file that fails to load is skipped; the
// first such error is returned beside the rows that did load.
func milestoneCostRows(v *core.Vault, ms string) ([]milestoneIssueRow, costTotal, error) {
	rows := []milestoneIssueRow{}
	var total costTotal
	paths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return rows, total, err
	}
	var firstErr error
	for _, p := range paths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("loading %s: %w", p, err)
			}
			continue
		}
		if milestoneSlug(a.FrontMatter["milestone"]) != ms {
			continue
		}
		status, _ := a.FrontMatter["status"].(string)
		rows = append(rows, milestoneIssueRow{ID: listIDFor(core.TypeIssue, p), Status: status, Cost: costFromFrontMatter(a.FrontMatter)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	total.Issues = len(rows)
	for _, r := range rows {
		if r.Cost == nil {
			continue
		}
		total.Costed++
		total.Rounds += r.Cost.Rounds
		total.Diff += r.Cost.Diff
		total.Files += r.Cost.Files
		total.Tokens += r.Cost.Tokens
	}
	return rows, total, firstErr
}

// unfinished reports whether the row's issue is neither resolved nor abandoned.
func (r milestoneIssueRow) unfinished() bool {
	return r.Status != "resolved" && r.Status != "abandoned"
}

func costFromFrontMatter(fm map[string]any) *issueCost {
	var vals [4]int
	for i, k := range []string{"cost_rounds", "cost_diff", "cost_files", "cost_tokens"} {
		n, ok := fm[k].(int)
		if !ok {
			return nil
		}
		vals[i] = n
	}
	return &issueCost{vals[0], vals[1], vals[2], vals[3]}
}

func (r milestoneIssueRow) line() string {
	cost := "—"
	if c := r.Cost; c != nil {
		cost = fmt.Sprintf("%dr %dl %df %dt", c.Rounds, c.Diff, c.Files, c.Tokens)
	}
	return fmt.Sprintf("%s\t%s\t%s", r.ID, r.Status, cost)
}
