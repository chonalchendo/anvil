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
	// Callers pass a user-typed slug (`--milestone anvil.v0-2`); artifacts.id
	// and links.target are both canonical, so normalise before either lookup.
	milestoneID = core.CanonicalID(core.TypeMilestone, milestoneID)
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

// MilestoneChildren is the per-status breakdown of issues linked to a
// milestone via the `milestone` frontmatter slot, so a queue view can show
// the derived progress next to the stored status.
type MilestoneChildren struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Resolved   int `json:"resolved"`
	Abandoned  int `json:"abandoned"`
	Total      int `json:"total"`
}

// MilestoneChildren counts a milestone's linked issues by status. An
// unresolvable milestoneID (typo, or a non-milestone id) is the caller's
// concern — this always reports zero counts rather than erroring, since the
// counts are advisory display data, not a gate.
func (d *DB) MilestoneChildren(milestoneID string) (MilestoneChildren, error) {
	milestoneID = core.CanonicalID(core.TypeMilestone, milestoneID)
	const q = `
SELECT
    COUNT(CASE WHEN a.status = 'open' THEN 1 END),
    COUNT(CASE WHEN a.status = 'in-progress' THEN 1 END),
    COUNT(CASE WHEN a.status = 'resolved' THEN 1 END),
    COUNT(CASE WHEN a.status = 'abandoned' THEN 1 END),
    COUNT(*)
FROM links l
JOIN artifacts a ON a.id = l.source AND a.type = 'issue'
WHERE l.relation = 'milestone' AND l.target = ?`
	var mc MilestoneChildren
	if err := d.sql.QueryRow(q, milestoneID).Scan(&mc.Open, &mc.InProgress, &mc.Resolved, &mc.Abandoned, &mc.Total); err != nil {
		return MilestoneChildren{}, fmt.Errorf("milestone children %s: %w", milestoneID, err)
	}
	return mc, nil
}

// MilestoneStale reports whether every linked issue is caught up (resolved or
// abandoned) but the milestone's own stored status hasn't caught up to done —
// the drift anvil.0275 makes visible in list/show milestone. A milestone with
// no linked issues is never stale, and a bucket milestone is never stale:
// buckets are rolling trackers with no terminal done state (doctor.go's
// checkFinishedMilestone carve-out, and the transition table refuses
// planned/in-progress → done for kind: bucket), so "all children caught up"
// never implies drift for one.
func MilestoneStale(mc MilestoneChildren, status, kind string) bool {
	if kind == "bucket" {
		return false
	}
	caughtUp := mc.Resolved + mc.Abandoned
	return mc.Total > 0 && caughtUp == mc.Total && status != "done"
}
