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

// AppendExternalLink appends uri to the source artifact's `external_links`
// array. Idempotent: re-appending an identical uri is a no-op. Does not touch
// `related[]` — external pointers live in their own field so the link-graph
// indexer (which parses `related[]` as wikilinks) never tries to resolve them.
func AppendExternalLink(v *Vault, src Type, srcID, uri string) error {
	path := filepath.Join(v.Root, src.Dir(), srcID+".md")
	a, err := LoadArtifact(path)
	if err != nil {
		return fmt.Errorf("load source: %w", err)
	}
	existing, _ := a.FrontMatter["external_links"].([]any)
	for _, e := range existing {
		if s, ok := e.(string); ok && s == uri {
			return nil
		}
	}
	a.FrontMatter["external_links"] = append(existing, uri)
	return a.Save()
}
