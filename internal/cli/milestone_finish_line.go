package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
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
// block runner (create_feasibility.go), in dir, so both gates share one execution shape.
// A bare command resolves the checkout's bin/ first on PATH; call the installed
// binary by absolute path.
func runAcceptance(cmd *cobra.Command, m *core.Artifact, dir string) []acceptanceResult {
	preds, _ := m.FrontMatter["acceptance"].([]any)
	results := make([]acceptanceResult, 0, len(preds))
	for i, p := range preds {
		s, _ := p.(string)
		cmd.PrintErrln(fmt.Sprintf("anvil: running acceptance predicate %d in this environment %s", i+1, runnerEnvNote))
		r := runFeasibilityBlock(s, dir, feasibilityTimeout)
		_, failed := r.failure()
		res := acceptanceResult{
			Criterion: s,
			Met:       !failed,
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

// milestoneAt names the ref a `milestone status --at` run measured and the
// commit it resolved to.
type milestoneAt struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

// runAcceptanceAt runs acceptance on a detached checkout of ref in the project
// repo, so team mode can measure a stack top. It returns the resolved commit.
// The checkout is removed after.
func runAcceptanceAt(cmd *cobra.Command, m *core.Artifact, id string, fl finishLine, ref string) ([]acceptanceResult, string, error) {
	if fl.Dir == "" {
		cause := "the milestone has no project"
		if fl.Err != nil {
			cause = fl.Err.Error()
		}
		return nil, "", errfmt.NewStructured("milestone_status_at_no_repo").
			Set("message", "--at needs the project repo, which did not resolve for "+id+": "+cause).
			Set("fix_hint", "set the milestone project to one with a resolvable repo, then re-run anvil milestone status "+id+" --at "+ref)
	}
	dir, commit, cleanup, err := checkoutAt(cmd.ErrOrStderr(), fl.Dir, ref, "anvil milestone status "+id+" --at "+ref)
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	return runAcceptance(cmd, m, dir), commit, nil
}
