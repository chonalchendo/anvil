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
// vault.db on first use (no stamp = run a full reindex). Returns
// errfmt.IndexStale if the vault has been edited externally since last
// reindex.
func indexAfterSave(v *core.Vault, a *core.Artifact) error {
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		return fmt.Errorf("opening index: %w", err)
	}
	defer db.Close()

	if err := db.CheckFreshness(v.Root); err != nil {
		switch {
		case errors.Is(err, index.ErrLastReindexUnset):
			if _, err := db.Reindex(v.Root); err != nil {
				return fmt.Errorf("bootstrap reindex: %w", err)
			}
		case errors.Is(err, index.ErrIndexStale):
			return errfmt.NewIndexStale()
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
