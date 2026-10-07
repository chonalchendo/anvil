package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// advanceMilestoneOnClaim moves the claimed issue's parent milestone from
// planned to in-progress on its first child claim, so list/show milestone
// reflect that work has started without a separate operator step (anvil.0275).
// Silent no-op when the issue carries no milestone, the milestone file can't
// be resolved, or the milestone isn't planned (already in-progress, done, or
// abandoned) — this is a derived-state follow, not a required edge. The
// approval gate still applies: a refused milestone stays planned and a
// stderr warning names the refusal code.
func advanceMilestoneOnClaim(v *core.Vault, claimed *core.Artifact, errW io.Writer) error {
	ms := milestoneSlug(claimed.FrontMatter["milestone"])
	if ms == "" {
		return nil
	}
	_, msPath, err := core.ResolveArtifact(v, core.TypeMilestone, ms)
	if err != nil {
		return nil //nolint:nilerr // best-effort follow
	}
	m, err := core.LoadArtifact(msPath)
	if err != nil {
		return nil //nolint:nilerr // best-effort follow
	}
	if status, _ := m.FrontMatter["status"].(string); status != "planned" {
		return nil
	}
	if gerr := gateMilestoneApproval(v, m, "milestone."+ms); gerr != nil {
		code := "unknown"
		var se *errfmt.Structured
		if errors.As(gerr, &se) {
			code = se.Code
		}
		fmt.Fprintf(errW, "warning: milestone %s stays planned: approval gate refused (%s)\n", ms, code)
		return nil
	}
	m.FrontMatter["status"] = "in-progress"
	m.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
	if err := m.Save(); err != nil {
		return fmt.Errorf("saving milestone %s: %w", ms, err)
	}
	return indexAfterSave(v, m)
}
