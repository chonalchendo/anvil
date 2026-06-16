package telemetry

import (
	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// PersistTraces writes task outcomes from a completed build run into the vault
// index so they are queryable by `anvil export traces`. Dry-run tasks are
// omitted (they carry no real outcome). This is the persistence seam named by
// build.TaskOutcome — the build command calls it after each run.
func PersistTraces(db *index.DB, p *core.Plan, sum *build.Summary) error {
	for _, task := range p.Tasks {
		oc, ok := sum.Outcomes[task.ID]
		if !ok || oc.Outcome == "skipped_dry_run" {
			continue
		}
		tr := index.Trace{
			TaskID:  oc.TaskID,
			Prompt:  oc.Prompt,
			Outcome: oc.Outcome,
			Model:   oc.Model,
			Effort:  oc.Effort,
		}
		if err := db.InsertTrace(tr); err != nil {
			return err
		}
	}
	return nil
}
