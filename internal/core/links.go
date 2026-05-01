package core

import (
	"fmt"
	"path/filepath"
)

// AppendLink appends a wikilink to (tgt, tgtID) onto the source artifact's
// `related` array. Structural slots (issue.milestone, plan.issue,
// milestone.product_design, milestone.system_design) are written via
// `anvil set`, not `link`. `link` is the associative verb.
func AppendLink(v *Vault, src Type, srcID string, tgt Type, tgtID string) error {
	path := filepath.Join(v.Root, src.Dir(), srcID+".md")
	a, err := LoadArtifact(path)
	if err != nil {
		return fmt.Errorf("load source: %w", err)
	}
	wikilink := fmt.Sprintf("[[%s.%s]]", tgt, tgtID)
	existing, _ := a.FrontMatter["related"].([]any)
	for _, e := range existing {
		if s, ok := e.(string); ok && s == wikilink {
			return nil
		}
	}
	a.FrontMatter["related"] = append(existing, wikilink)
	return a.Save()
}
