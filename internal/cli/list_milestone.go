package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

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
func enrichMilestoneItem(db *index.DB, item *listItem, id, status string) error {
	if db == nil {
		return nil
	}
	mc, err := db.MilestoneChildren(id)
	if err != nil {
		return err
	}
	stale := index.MilestoneStale(mc, status)
	item.Children = &mc
	item.Stale = &stale
	return nil
}

// measurementStaleWarning is the one-line list/show warning for a milestone
// whose Status block has aged past core.MeasurementStaleDays.
func measurementStaleWarning(id string) string {
	return fmt.Sprintf("warning: %s Status block was measured over %d days ago; re-measure and update its Measured: line", id, core.MeasurementStaleDays)
}

// flagMeasurementStale sets item.MeasurementStale for a milestone row when
// the verdict applies (scoped, in-progress, dated); otherwise it stays nil
// and the key is omitted. The stderr warning is emitted separately, after
// --limit truncation, by warnMeasurementStale.
func flagMeasurementStale(item *listItem, a *core.Artifact) {
	if item.Type != string(core.TypeMilestone) {
		return
	}
	if ms, ok := core.MeasurementStale(a, time.Now()); ok {
		item.MeasurementStale = &ms
	}
}

// warnMeasurementStale warns on stderr for each returned item flagged stale.
func warnMeasurementStale(cmd *cobra.Command, items []listItem) {
	for _, it := range items {
		if it.MeasurementStale != nil && *it.MeasurementStale {
			cmd.PrintErrln(measurementStaleWarning(it.ID))
		}
	}
}
