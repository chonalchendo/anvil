package index

import (
	"fmt"

	"github.com/chonalchendo/anvil/internal/core"
)

// MilestoneStatus is a milestone's derived done-signal: how many of the issues
// linked to it (via the `milestone` frontmatter slot) are resolved out of the
// total, and whether every one is. Done is the build loop's exit predicate; a
// milestone with no linked issues is never done.
type MilestoneStatus struct {
	Milestone string `json:"milestone"`
	Resolved  int    `json:"resolved"`
	Total     int    `json:"total"`
	Done      bool   `json:"done"`
}

// MilestoneStatus derives a milestone's done-signal from the status of the
// issues linked to it via the `milestone` slot (relation 'milestone'). Done is
// true only when the milestone has at least one linked issue and every one is
// resolved, so an empty milestone reports done=false. Returns
// ErrArtifactNotInIndex when the id is not a milestone in the index, so a typo —
// or a non-milestone id — surfaces rather than reporting a silent done=false.
func (d *DB) MilestoneStatus(milestoneID string) (MilestoneStatus, error) {
	a, err := d.GetArtifact(milestoneID)
	if err != nil {
		return MilestoneStatus{}, err
	}
	if a.Type != string(core.TypeMilestone) {
		return MilestoneStatus{}, ErrArtifactNotInIndex
	}
	const q = `
SELECT
    COUNT(*),
    COUNT(CASE WHEN a.status = 'resolved' THEN 1 END)
FROM links l
JOIN artifacts a ON a.id = l.source AND a.type = 'issue'
WHERE l.relation = 'milestone' AND l.target = ?`
	var total, resolved int
	if err := d.sql.QueryRow(q, milestoneID).Scan(&total, &resolved); err != nil {
		return MilestoneStatus{}, fmt.Errorf("milestone status %s: %w", milestoneID, err)
	}
	return MilestoneStatus{
		Milestone: milestoneID,
		Resolved:  resolved,
		Total:     total,
		Done:      total > 0 && resolved == total,
	}, nil
}
