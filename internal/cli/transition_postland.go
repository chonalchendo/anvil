package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

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
		r := runFeasibilityBlock(block, "", feasibilityTimeout)
		reason, failed := r.failure()
		if !failed {
			fmt.Fprintf(errW, "post-land %s passed\n", id)
			continue
		}
		fmt.Fprintf(errW, "warning: post-land %s is RED (%s); issue still resolves, re-check by hand\n%s\n", id, reason, r.output)
		red = append(red, id)
	}
	return red
}

// postLandRedNote is the issue-body suffix recording red post-land blocks; the
// body is not yet saved, so the note rides the single Save() after landing.
func postLandRedNote(red []string, body string) string {
	if len(red) == 0 {
		return ""
	}
	note := "\n> post-land RED (" + strings.Join(red, ", ") + "): issue resolved anyway; re-check by hand\n"
	if !strings.HasSuffix(body, "\n") {
		note = "\n" + note
	}
	return note
}
