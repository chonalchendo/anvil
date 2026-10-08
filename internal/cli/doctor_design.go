package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// checkDesignDrift gathers the design-vs-reality checks. It stays a wrapper
// because doctor.go sits at the 500-line cap; later checks append here.
func checkDesignDrift(v *core.Vault, projectSlug string) ([]doctorFinding, error) {
	return checkCandidateMilestoneDone(v, projectSlug)
}

var leadingLink = regexp.MustCompile(`^\s*\[\[[^\]]*\]\]`)

type milestoneRef struct{ id, status, done string }

// checkCandidateMilestoneDone flags each done milestone that a top-level bullet
// of the product design's ## Milestones section names, by link or by title.
// Decision item 4 says a done milestone leaves the list. Nested bullets are
// evidence, never candidates.
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
	bullets := topLevelBullets(core.Section(pd.Body, "Milestones"))
	if len(bullets) == 0 {
		return nil, nil
	}
	byID, byTitle, err := projectMilestones(v, projectSlug)
	if err != nil {
		return nil, err
	}
	var findings []doctorFinding
	seen := map[string]bool{}
	for _, bullet := range bullets {
		var refs []milestoneRef
		for _, target := range core.BodyWikilinkTargetsOfType(bullet, core.TypeMilestone) {
			if m, ok := byID[target]; ok {
				refs = append(refs, m)
			}
		}
		text := strings.ToLower(strings.TrimSpace(leadingLink.ReplaceAllString(bullet, "")))
		for title, m := range byTitle {
			// Prefix match: a title may itself contain "." (v0.1), so the
			// boundary is the character after the title, not a cut.
			if rest, ok := strings.CutPrefix(text, title); ok && (rest == "" || rest[0] == '.' || rest[0] == ':') {
				refs = append(refs, m)
			}
		}
		for _, m := range refs {
			if m.status != "done" || seen[m.id] {
				continue
			}
			seen[m.id] = true
			done := m.done
			if done == "" {
				done = "date not stamped"
			}
			findings = append(findings, doctorFinding{
				Kind:     "candidate-milestone-done",
				ID:       m.id,
				Evidence: fmt.Sprintf("%s ## Milestones lists %s (done %s)", pdID, m.id, done),
				Fix:      fmt.Sprintf("edit %s: remove the entry under ## Milestones", pdID),
			})
		}
	}
	return findings, nil
}

// topLevelBullets returns the text of each unindented `- ` line.
func topLevelBullets(section string) []string {
	var out []string
	for _, line := range strings.Split(section, "\n") {
		if text, ok := strings.CutPrefix(line, "- "); ok {
			out = append(out, text)
		}
	}
	return out
}

// projectMilestones indexes the project's milestones by id and by lowercased title.
func projectMilestones(v *core.Vault, projectSlug string) (byID, byTitle map[string]milestoneRef, err error) {
	paths, err := collectArtifactPaths(v.Root, core.TypeMilestone)
	if err != nil {
		return nil, nil, fmt.Errorf("reading milestones: %w", err)
	}
	byID, byTitle = map[string]milestoneRef{}, map[string]milestoneRef{}
	for _, p := range paths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue // unreadable: validate's domain
		}
		if proj, _ := a.FrontMatter["project"].(string); proj != projectSlug {
			continue
		}
		status, _ := a.FrontMatter["status"].(string)
		done, _ := a.FrontMatter["done"].(string)
		m := milestoneRef{core.CanonicalID(core.TypeMilestone, strings.TrimSuffix(filepath.Base(p), ".md")), status, done}
		byID[m.id] = m
		if title, _ := a.FrontMatter["title"].(string); title != "" {
			byTitle[strings.ToLower(title)] = m
		}
	}
	return byID, byTitle, nil
}
