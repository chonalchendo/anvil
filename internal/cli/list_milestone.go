package cli

import (
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
