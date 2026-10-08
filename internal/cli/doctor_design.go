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
	findings, err := checkCandidateMilestoneDone(v, projectSlug)
	if err != nil {
		return nil, err
	}
	untouched, err := checkDesignUntouchedAfterMilestone(v)
	if err != nil {
		return nil, err
	}
	findings = append(findings, untouched...)
	root, _ := gitRepoRootFn() // no repo → empty root → no code-ref findings
	return append(findings, checkDesignCodeRefMissing(v, projectSlug, root)...), nil
}

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
		text := strings.ToLower(strings.TrimSpace(bullet))
		for title, m := range byTitle {
			// Prefix match: a title may itself contain "." (v0.1), so the
			// boundary is the character after the title, not a cut.
			if rest, ok := strings.CutPrefix(text, title); ok && (rest == "" || rest[0] == ':' || rest == "." || strings.HasPrefix(rest, ". ")) {
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

// checkDesignUntouchedAfterMilestone flags a design that a done milestone links
// but whose updated date predates the milestone's done date. A done milestone
// without a done date is not examined.
func checkDesignUntouchedAfterMilestone(v *core.Vault) ([]doctorFinding, error) {
	paths, err := collectArtifactPaths(v.Root, core.TypeMilestone)
	if err != nil {
		return nil, fmt.Errorf("reading milestones: %w", err)
	}
	var findings []doctorFinding
	for _, p := range paths {
		m, err := core.LoadArtifact(p)
		if err != nil {
			continue // unreadable: validate's domain
		}
		if status, _ := m.FrontMatter["status"].(string); status != "done" {
			continue
		}
		done, _ := m.FrontMatter["done"].(string)
		if done == "" {
			continue
		}
		mID := listIDFor(core.TypeMilestone, p)
		for _, d := range linkedDesigns(m.FrontMatter) {
			_, dPath, err := core.ResolveArtifact(v, d.t, d.target)
			if err != nil {
				return nil, err
			}
			design, err := core.LoadArtifact(dPath)
			if err != nil {
				continue // dangling link: validate's domain
			}
			updated, _ := design.FrontMatter["updated"].(string)
			if updated == "" || updated >= done { // ISO dates order lexically
				continue
			}
			dID := core.CanonicalID(d.t, d.target)
			findings = append(findings, doctorFinding{
				Kind:     "design-untouched-after-milestone",
				ID:       dID,
				Evidence: fmt.Sprintf("milestone %s done %s; %s updated %s", mID, done, dID, updated),
				Fix:      fmt.Sprintf("revise %s (set updated), or unlink it from %s", dID, mID),
			})
		}
	}
	return findings, nil
}

type designLink struct {
	t      core.Type
	target string
}

// linkedDesigns lists the distinct designs a milestone's frontmatter links.
func linkedDesigns(fm map[string]any) []designLink {
	var out []designLink
	seen := map[string]bool{}
	add := func(t core.Type, raw string) {
		target := core.BareID(t, raw)
		if key := string(t) + "." + target; target != "" && !seen[key] {
			seen[key] = true
			out = append(out, designLink{t, target})
		}
	}
	for _, f := range []struct {
		field string
		t     core.Type
	}{{"product_design", core.TypeProductDesign}, {"system_design", core.TypeSystemDesign}} {
		if s, _ := fm[f.field].(string); s != "" {
			add(f.t, s)
		}
	}
	rel, _ := fm["related"].([]any)
	for _, r := range rel {
		s, _ := r.(string)
		s = core.UnwrapWikilink(s)
		for _, t := range []core.Type{core.TypeComponentDesign, core.TypeSystemDesign, core.TypeProductDesign} {
			if strings.HasPrefix(s, string(t)+".") {
				add(t, s)
			}
		}
	}
	return out
}

var (
	backtickToken = regexp.MustCompile("`([^`\\n]+)`")
	repoPathToken = regexp.MustCompile(`^[A-Za-z0-9_./-]+\.(go|md|sh|json|ya?ml|tmpl|toml)$`)
)

// checkDesignCodeRefMissing flags a backticked repo path in a design body of
// the project that does not exist under repoRoot. An empty root examines nothing.
func checkDesignCodeRefMissing(v *core.Vault, projectSlug, repoRoot string) []doctorFinding {
	if projectSlug == "" || repoRoot == "" {
		return nil
	}
	var findings []doctorFinding
	for _, t := range []core.Type{core.TypeComponentDesign, core.TypeSystemDesign, core.TypeProductDesign} {
		paths, err := collectArtifactPaths(v.Root, t)
		if err != nil {
			continue
		}
		for _, p := range paths {
			a, err := core.LoadArtifact(p)
			if err != nil {
				continue // unreadable: validate's domain
			}
			if proj, _ := a.FrontMatter["project"].(string); proj != projectSlug {
				continue
			}
			id := listIDFor(t, p)
			seen := map[string]bool{}
			for _, m := range backtickToken.FindAllStringSubmatch(a.Body, -1) {
				ref := m[1]
				if seen[ref] || !strings.Contains(ref, "/") || strings.ContainsAny(ref, "*<>{") || !repoPathToken.MatchString(ref) {
					continue
				}
				seen[ref] = true
				if _, err := os.Stat(filepath.Join(repoRoot, ref)); err == nil {
					continue
				}
				findings = append(findings, doctorFinding{
					Kind:     "design-code-ref-missing",
					ID:       id,
					Evidence: fmt.Sprintf("%s not in %s", ref, repoRoot),
					Fix:      fmt.Sprintf("edit %s: fix or remove %s", id, ref),
				})
			}
		}
	}
	return findings
}
