package core

import (
	"fmt"
	"path/filepath"
)

// linkField maps (source type, target type) to the frontmatter field on the
// source that holds wikilinks to the target. Only the pairs supported in v0.1
// are present; all others are rejected.
var linkField = map[Type]map[Type]string{
	TypePlan:      {TypeMilestone: "milestones"},
	TypeMilestone: {TypeIssue: "issues", TypePlan: "plans"},
	TypeIssue:     {TypeMilestone: "related"},
	TypeDecision:  {TypeIssue: "related"},
}

// AppendLink appends a wikilink to tgtID onto the appropriate frontmatter
// field of the source artifact identified by (src, srcID). The operation is
// idempotent: if the wikilink already appears in the field it is not added
// again.
func AppendLink(v *Vault, src Type, srcID string, tgt Type, tgtID string) error {
	fields, ok := linkField[src]
	if !ok {
		return fmt.Errorf("link %s → %s not supported in v0.1", src, tgt)
	}
	field, ok := fields[tgt]
	if !ok {
		return fmt.Errorf("link %s → %s not supported in v0.1", src, tgt)
	}

	path := filepath.Join(v.Root, src.Dir(), srcID+".md")
	a, err := LoadArtifact(path)
	if err != nil {
		return fmt.Errorf("load source: %w", err)
	}

	wikilink := fmt.Sprintf("[[%s.%s]]", tgt, tgtID)
	existing, _ := a.FrontMatter[field].([]any)
	for _, e := range existing {
		if s, ok := e.(string); ok && s == wikilink {
			return nil // already present
		}
	}
	a.FrontMatter[field] = append(existing, wikilink)
	return a.Save()
}
