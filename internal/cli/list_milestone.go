package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// openMilestoneListIndex opens the index for milestone rows' derived
// children/stale enrichment. An unopenable index degrades to a nil *DB
// (warned on stderr) rather than failing the whole list — matching
// show.go's degraded path, since the vault scan the caller runs still works
// without the index. Returns nil for any type other than milestone.
func openMilestoneListIndex(cmd *cobra.Command, v *core.Vault, t core.Type) *index.DB {
	if t != core.TypeMilestone {
		return nil
	}
	db, dberr := indexForRead(v)
	if dberr != nil {
		cmd.PrintErrln("warning: milestone children: " + dberr.Error())
		return nil
	}
	return db
}

// enrichMilestoneItem populates item.Children/Stale from db — the derived
// issue-status breakdown for a milestone row, and whether the milestone's
// stored status has drifted behind it (anvil.0275). No-op when db is nil
// (index unopenable, or t != milestone).
func enrichMilestoneItem(db *index.DB, item *listItem, id, status, kind string) error {
	if db == nil {
		return nil
	}
	mc, err := db.MilestoneChildren(id)
	if err != nil {
		return err
	}
	stale := index.MilestoneStale(mc, status, kind)
	item.Children = &mc
	item.Stale = &stale
	return nil
}

// measurementStaleWarning is the one-line list/show warning for a milestone
// whose Status block has aged past core.MeasurementStaleDays.
func measurementStaleWarning(id string) string {
	return fmt.Sprintf("warning: %s Status block was measured over %d days ago; re-measure and update its Measured: line", id, core.MeasurementStaleDays)
}

// flagMeasurementStale sets item.MeasurementStale for a milestone row and
// warns on stderr when true; no-op for other types.
func flagMeasurementStale(cmd *cobra.Command, item *listItem, a *core.Artifact) {
	if item.Type != string(core.TypeMilestone) {
		return
	}
	ms := core.MeasurementStale(a, time.Now())
	item.MeasurementStale = &ms
	if ms {
		cmd.PrintErrln(measurementStaleWarning(item.ID))
	}
}
