package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// acceptanceResult is one milestone `acceptance:` predicate run on the current
// checkout. A timeout or a run failure counts as not met.
type acceptanceResult struct {
	Criterion string `json:"criterion"`
	Met       bool   `json:"met"`
	Exit      int    `json:"exit"`
	TimedOut  bool   `json:"timed_out,omitempty"`
	RunError  string `json:"run_error,omitempty"`
}

// detail is the human-readable measurement: the exit code, or why there is none.
func (r acceptanceResult) detail() string {
	switch {
	case r.TimedOut:
		return "timed out"
	case r.RunError != "":
		return r.RunError
	}
	return fmt.Sprintf("exit %d", r.Exit)
}

// runAcceptance runs each acceptance predicate through the issue create gate's
// block runner (create_feasibility.go), so both gates share one execution shape.
func runAcceptance(cmd *cobra.Command, m *core.Artifact) []acceptanceResult {
	preds, _ := m.FrontMatter["acceptance"].([]any)
	results := make([]acceptanceResult, 0, len(preds))
	for i, p := range preds {
		s, _ := p.(string)
		cmd.PrintErrln(fmt.Sprintf("anvil: running acceptance predicate %d in this environment (your privileges, cwd and environment; not sandboxed)", i+1))
		r := runFeasibilityBlock(s)
		res := acceptanceResult{
			Criterion: s,
			Met:       r.runErr == nil && !r.timedOut && r.exit == 0,
			Exit:      r.exit,
			TimedOut:  r.timedOut,
		}
		if r.runErr != nil {
			res.RunError = r.runErr.Error()
		}
		results = append(results, res)
	}
	return results
}

func unmetCriteria(rs []acceptanceResult) []string {
	var unmet []string
	for _, r := range rs {
		if !r.Met {
			unmet = append(unmet, r.Criterion)
		}
	}
	return unmet
}
