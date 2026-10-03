package cli

import (
	"fmt"
	"io"

	"github.com/chonalchendo/anvil/internal/core"
)

// runPostLandBlocks runs the issue's post-land-marked Indirect blocks once the
// PR is MERGED. A red block only warns: the merge is irreversible, so refusing
// to resolve would strand a landed issue while fixing nothing.
func runPostLandBlocks(errW io.Writer, body string) {
	blocks, err := core.VerificationBlocks(body, "Indirect")
	if err != nil {
		fmt.Fprintf(errW, "warning: post-land: cannot extract Indirect blocks: %v\n", err)
		return
	}
	for i, block := range blocks {
		if !core.IsPostLand(block) {
			continue
		}
		name := fmt.Sprintf("Indirect#%d", i+1)
		r := runFeasibilityBlock(block)
		switch {
		case r.runErr != nil:
			fmt.Fprintf(errW, "warning: post-land %s produced no exit status: %v\n", name, r.runErr)
		case r.timedOut:
			fmt.Fprintf(errW, "warning: post-land %s timed out after %s\n", name, feasibilityTimeout)
		case r.exit != 0:
			fmt.Fprintf(errW, "warning: post-land %s is RED (exit %d); the issue is still resolved, re-check it by hand\n%s\n", name, r.exit, r.output)
		default:
			fmt.Fprintf(errW, "post-land %s passed\n", name)
		}
	}
}
