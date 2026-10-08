package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/chonalchendo/anvil/internal/core"
)

// checkDesignDrift gathers the design-vs-reality checks; later checks append here.
func checkDesignDrift(v *core.Vault, projectSlug string) ([]doctorFinding, error) {
	return checkCandidateMilestoneDone(v, projectSlug)
}

// checkCandidateMilestoneDone flags each done milestone the product design's
// ## Milestones section still links; decision item 4 says a done milestone leaves the list.
func checkCandidateMilestoneDone(v *core.Vault, projectSlug string) ([]doctorFinding, error) {
	if projectSlug == "" {
		return nil, nil
	}
	_, pdPath, err := core.ResolveArtifact(v, core.TypeProductDesign, projectSlug)
	if err != nil {
		return nil, err
	}
	pdID := "product-design." + projectSlug
	pd, err := core.LoadArtifact(pdPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", pdID, err)
	}
	section := core.Section(pd.Body, "Milestones")
	var findings []doctorFinding
	for _, target := range core.BodyWikilinkTargetsOfType(section, core.TypeMilestone) {
		id, path, err := core.ResolveArtifact(v, core.TypeMilestone, target)
		if err != nil {
			return nil, err
		}
		m, err := core.LoadArtifact(path)
		if err != nil {
			continue // dangling link: validate's domain
		}
		if status, _ := m.FrontMatter["status"].(string); status != "done" {
			continue
		}
		done, _ := m.FrontMatter["done"].(string)
		if done == "" {
			done = "no date"
		}
		findings = append(findings, doctorFinding{
			Kind:     "candidate-milestone-done",
			ID:       id,
			Evidence: fmt.Sprintf("%s ## Milestones links %s (done %s)", pdID, id, done),
			Fix:      fmt.Sprintf("edit %s: remove the entry under ## Milestones", pdID),
		})
	}
	return findings, nil
}
