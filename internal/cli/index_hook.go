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
	defer db.Close()

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
	if err := db.ReplaceLinks(row.ID, index.LinkRowsFromFrontmatter(row.ID, a.FrontMatter)); err != nil {
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
				db.Close()
				return nil, fmt.Errorf("bootstrap reindex: %w", err)
			}
		case errors.Is(err, index.ErrIndexStale):
			db.Close()
			return nil, errfmt.NewIndexStale()
		default:
			db.Close()
			return nil, fmt.Errorf("freshness check: %w", err)
		}
	}
	return db, nil
}
