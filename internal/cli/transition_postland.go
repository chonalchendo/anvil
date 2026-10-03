package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// doLandPR merges via doLandPRMerge, then runs post-land blocks; only a merge failure aborts.
func doLandPR(errW io.Writer, a *core.Artifact, id string, prNum int, worktreeOverride string, localValidated bool) error {
	if err := doLandPRMerge(errW, a, id, prNum, worktreeOverride, localValidated); err != nil {
		return err
	}
	if red := runPostLandBlocks(errW, a.Body); len(red) > 0 {
		if !strings.HasSuffix(a.Body, "\n") {
			a.Body += "\n"
		}
		a.Body += "\n> post-land RED (" + strings.Join(red, ", ") + "): issue resolved anyway; re-check by hand\n"
	}
	return nil
}

// runPostLandBlocks runs post-land-marked Indirect blocks once, after MERGED,
// and returns the ids of the red ones. Red only warns: the merge is
// irreversible, so refusing to resolve would strand a landed issue. The blocks
// run in the main checkout, which --land-pr never pulls, so they must probe
// live state rather than the tree.
func runPostLandBlocks(errW io.Writer, body string) []string {
	blocks, err := core.VerificationBlocks(body, "Indirect")
	if err != nil {
		fmt.Fprintf(errW, "warning: post-land: %v\n", err)
	}
	var red []string
	for i, block := range blocks {
		if !core.IsPostLand(block) {
			continue
		}
		id := fmt.Sprintf("Indirect#%d", i+1)
		r := runFeasibilityBlock(block)
		var reason string
		switch {
		case r.timedOut:
			reason = "timed out"
		case r.runErr != nil:
			reason = r.runErr.Error()
		case r.exit != 0:
			reason = fmt.Sprintf("exit %d", r.exit)
		default:
			fmt.Fprintf(errW, "post-land %s passed\n", id)
			continue
		}
		fmt.Fprintf(errW, "warning: post-land %s is RED (%s); issue still resolves, re-check by hand\n%s\n", id, reason, r.output)
		red = append(red, id)
	}
	return red
}
