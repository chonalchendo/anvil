package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// indexAfterSave updates vault.db for the artifact at a.Path. Bootstraps
// vault.db on first use (no stamp = run a full reindex). Auto-reindexes on
// detected external drift so sequential writes in one process don't need
// a manual `anvil reindex` between them; the verb stays for the read path
// and for full rebuilds.
func indexAfterSave(v *core.Vault, a *core.Artifact) error {
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		return fmt.Errorf("opening index: %w", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	// Skip the file we just wrote — its mtime is by definition newer than
	// the previous stamp, but it's a write we initiated, not external drift.
	if err := db.CheckFreshnessExcept(v.Root, a.Path); err != nil {
		switch {
		case errors.Is(err, index.ErrLastReindexUnset):
			if _, err := db.Reindex(v.Root); err != nil {
				return fmt.Errorf("bootstrap reindex: %w", err)
			}
		case errors.Is(err, index.ErrIndexStale):
			// External drift since the last stamp — absorb it via a
			// full reindex (which also picks up our just-saved file)
			// instead of forcing the caller to `anvil reindex` first.
			if _, err := db.Reindex(v.Root); err != nil {
				return fmt.Errorf("auto-reindex on stale: %w", err)
			}
		default:
			return fmt.Errorf("freshness check: %w", err)
		}
	}

	row, err := index.ArtifactRowFromFrontmatter(a.FrontMatter, a.Path)
	if err != nil {
		return fmt.Errorf("extracting row: %w", err)
	}
	if err := db.UpsertArtifact(row); err != nil {
		return err
	}
	links := append(index.LinkRowsFromFrontmatter(row.ID, a.FrontMatter), index.LinkRowsFromBody(row.ID, a.Body)...)
	if err := db.ReplaceLinks(row.ID, links); err != nil {
		return err
	}
	// Keep artifact_fts in lockstep with the just-saved row so a later create's
	// content-dedup query (which reads artifact_fts) sees this issue/milestone
	// without relying on an incidental reindex. No-op for non-issue/milestone.
	if err := db.IndexArtifactFTS(row, a.FrontMatter); err != nil {
		return err
	}
	// Keep the tags table in lockstep too, so `anvil index` finds the just-saved
	// artifact's facets without a manual reindex. Skipping this would leave the
	// row tag-less until a full reindex — incremental can't catch it, since the
	// re-stamp below pushes last_reindex past this file's mtime.
	if err := db.ReplaceTags(row.ID, index.TagsFromFrontmatter(a.FrontMatter)); err != nil {
		return err
	}
	// Keep learning_fts in lockstep too, so a just-saved learning's TL;DR is
	// content-searchable (`anvil list learning --search`) without a manual
	// reindex. Without this the row stays unsearchable until a full rebuild —
	// incremental can't catch it, since the re-stamp below pushes last_reindex
	// past this file's mtime. No-op for non-learning types.
	if err := db.IndexLearningFTS(row, a.Body); err != nil {
		return err
	}

	// Re-stamp last-reindex so the file we just wrote (which advanced the vault
	// dir mtime) doesn't immediately make the index look stale on subsequent
	// reads.
	return db.SetLastReindex(time.Now())
}

// indexForRead opens vault.db, returns IndexStale if the vault drifted, and
// bootstraps on first read. Caller is responsible for Close().
func indexForRead(v *core.Vault) (*index.DB, error) {
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		return nil, fmt.Errorf("opening index: %w", err)
	}
	if err := db.CheckFreshness(v.Root); err != nil {
		switch {
		case errors.Is(err, index.ErrLastReindexUnset):
			if _, err := db.Reindex(v.Root); err != nil {
				db.Close() //nolint:errcheck,gosec // close in defer; error not actionable
				return nil, fmt.Errorf("bootstrap reindex: %w", err)
			}
		case errors.Is(err, index.ErrIndexStale):
			db.Close() //nolint:errcheck,gosec // close in defer; error not actionable
			return nil, errfmt.NewIndexStale()
		default:
			db.Close() //nolint:errcheck,gosec // close in defer; error not actionable
			return nil, fmt.Errorf("freshness check: %w", err)
		}
	}
	return db, nil
}
