package cli

import (
	"fmt"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/hydrate"
)

// checkSystemDesignParentMissing flags a dotted system-design id whose parent
// design (the id minus its last segment) does not exist. Hydrate skips the
// gap silently, so doctor is where it surfaces.
func checkSystemDesignParentMissing(v *core.Vault, projectSlug string) ([]doctorFinding, error) {
	if projectSlug == "" {
		return nil, nil
	}
	paths, err := collectArtifactPaths(v.Root, core.TypeSystemDesign)
	if err != nil {
		return nil, fmt.Errorf("reading system designs: %w", err)
	}
	var findings []doctorFinding
	for _, p := range paths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue // unreadable: validate's domain
		}
		if proj, _ := a.FrontMatter["project"].(string); proj != projectSlug {
			continue
		}
		id := core.WikilinkTarget(core.TypeSystemDesign, listIDFor(core.TypeSystemDesign, p))
		parent := hydrate.SystemDesignParent(core.BareID(core.TypeSystemDesign, id))
		if parent == "" {
			continue
		}
		parentID := core.WikilinkTarget(core.TypeSystemDesign, parent)
		if core.WikilinkTargetExists(v, parentID) {
			continue
		}
		findings = append(findings, doctorFinding{
			Kind:     "system-design-parent-missing",
			ID:       id,
			Evidence: fmt.Sprintf("%s names parent %s, which does not exist", id, parentID),
			Fix:      fmt.Sprintf("create %s, or rename %s", parentID, id),
		})
	}
	return findings, nil
}
