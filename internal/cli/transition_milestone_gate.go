package cli

import (
	"fmt"
	"slices"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// unreadInbox lists the raw inbox items that link milestone id, by a body
// wikilink or any frontmatter slot. A load failure is an error: a partial
// list could let a milestone through with routed items unread.
func unreadInbox(v *core.Vault, id string) ([]string, error) {
	paths, err := collectArtifactPaths(v.Root, core.TypeInbox)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, p := range paths {
		item, err := core.LoadArtifact(p)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", p, err)
		}
		if status, _ := item.FrontMatter["status"].(string); status != "raw" {
			continue
		}
		if slices.Contains(linkTargetsOfType(item, core.TypeMilestone), id) {
			ids = append(ids, listIDFor(core.TypeInbox, p))
		}
	}
	return ids, nil
}

// gateMilestoneApproval refuses `transition milestone in-progress` from
// planned while a scoped milestone's form is incomplete or an open inbox item
// still links it. On success it stamps the approval date; the caller saves.
func gateMilestoneApproval(v *core.Vault, m *core.Artifact, id string) error {
	if label, code, missing := core.MissingMilestoneFormPart(m); missing {
		return errfmt.NewStructured(code).
			Set("milestone", id).
			Set("fix_hint", "add a line starting "+label+"** (under ## Objective), then retry")
	}
	unread, err := unreadInbox(v, id)
	if err != nil {
		return errfmt.NewStructured("milestone_scan_failed").
			Set("milestone", id).
			Set("error", err.Error()).
			Set("fix_hint", "fix the unreadable inbox artifact, then retry")
	}
	if len(unread) > 0 {
		return errfmt.NewStructured("inbox_unread").
			Set("milestone", id).
			Set("inbox", unread).
			Set("fix_hint", "promote or drop each routed inbox item (anvil transition inbox <id> promoted|dropped), then retry")
	}
	m.FrontMatter["approved"] = time.Now().UTC().Format("2006-01-02")
	return nil
}
