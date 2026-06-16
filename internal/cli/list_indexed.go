package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// runListIndexed serves --ready/--orphans from the index DB rather than walking
// the filesystem. Severity and milestone are not in the index; filter post-load.
// Load all matching rows before applying the limit so the truncation hint
// reflects the filtered total, not the pre-filter row count.
func runListIndexed(cmd *cobra.Command, t core.Type, ready, orphans bool, f listFilters, asJSON bool, limit int, fields []string) error {
	if ready && t != core.TypeIssue {
		e := errfmt.NewUnsupportedForType(string(t), []string{"issue"})
		return printAndReturn(cmd, e)
	}
	v, err := core.ResolveVault()
	if err != nil {
		return fmt.Errorf("resolving vault: %w", err)
	}
	db, err := indexForRead(v)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	qf := index.QueryFilters{
		Status: f.Status, Project: f.Project,
		Since: f.Since, Until: f.Until,
	}
	var rows []index.ArtifactRow
	switch {
	case ready:
		rows, err = db.ListReady(string(t), qf)
	case orphans:
		rows, err = db.ListOrphans(qf)
	}
	if err != nil {
		return err
	}
	items := indexRowsToItems(rows, f)
	total := len(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return emitList(cmd, items, total, asJSON, t, fields)
}

// indexRowsToItems enriches index rows with frontmatter fields the index does
// not store (title, description, severity, milestone, tags), applying the
// post-load severity/milestone/invalid-body filters. Input order is preserved
// — callers relying on a query's ORDER BY (e.g. FTS rank) keep that order.
func indexRowsToItems(rows []index.ArtifactRow, f listFilters) []listItem {
	items := make([]listItem, 0, len(rows))
	for _, r := range rows {
		item := listItem{
			ID: r.ID, Type: r.Type, Status: r.Status,
			Project: r.Project, Path: r.Path, Created: r.Created,
		}
		if a, err := core.LoadArtifact(r.Path); err == nil {
			item.Title, _ = a.FrontMatter["title"].(string)
			item.Description, _ = a.FrontMatter["description"].(string)
			item.Severity, _ = a.FrontMatter["severity"].(string)
			item.Milestone = milestoneSlug(a.FrontMatter["milestone"])
			item.Tags = stringTags(a.FrontMatter["tags"])
			if f.InvalidBody {
				errs := core.ValidateIssue(a)
				if len(errs) == 0 {
					continue // valid body — exclude from --invalid-body results
				}
				item.MissingSection = firstMissingSection(errs)
			}
		}
		if f.Severity != "" && item.Severity != f.Severity {
			continue
		}
		if f.Milestone != "" && item.Milestone != f.Milestone {
			continue
		}
		items = append(items, item)
	}
	return items
}

// runListSearch runs an FTS content search over learning TL;DRs, emitting hits
// in FTS rank order. Only the learning type is searchable.
func runListSearch(cmd *cobra.Command, t core.Type, query string, f listFilters, asJSON bool, limit int, fields []string) error {
	v, err := core.ResolveVault()
	if err != nil {
		return fmt.Errorf("resolving vault: %w", err)
	}
	db, err := indexForRead(v)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// Load all matches, then truncate after enrichment so total reflects the
	// real match count and the truncation hint fires (mirrors runListIndexed).
	rows, err := db.SearchLearnings(query, index.QueryFilters{
		Status: f.Status, Project: f.Project,
		Since: f.Since, Until: f.Until,
	})
	if err != nil {
		return err
	}
	items := indexRowsToItems(rows, f)
	total := len(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return emitList(cmd, items, total, asJSON, t, fields)
}
