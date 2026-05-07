package index

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// ReindexStats summarises a Reindex call.
type ReindexStats struct {
	Artifacts  int
	Links      int
	DurationMS int64
}

// Reindex tears down both tables and walks the vault to repopulate them.
// Stamps the last-reindex time on success.
func (d *DB) Reindex(vaultRoot string) (ReindexStats, error) {
	start := time.Now()
	tx, err := d.sql.Begin()
	if err != nil {
		return ReindexStats{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM links`); err != nil {
		return ReindexStats{}, fmt.Errorf("clear links: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM artifacts`); err != nil {
		return ReindexStats{}, fmt.Errorf("clear artifacts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ReindexStats{}, fmt.Errorf("commit clear: %w", err)
	}

	var stats ReindexStats
	walkErr := filepath.WalkDir(vaultRoot, func(path string, dEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dEntry.IsDir() {
			if dEntry.Name() == ".anvil" || strings.HasPrefix(dEntry.Name(), ".") && path != vaultRoot {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		a, err := core.LoadArtifact(path)
		if err != nil {
			return nil // ignore unparseable files; reindex is best-effort
		}
		row, err := ArtifactRowFromFrontmatter(a.FrontMatter, path)
		if err != nil {
			return nil
		}
		if err := d.UpsertArtifact(row); err != nil {
			return err
		}
		stats.Artifacts++
		links := LinkRowsFromFrontmatter(row.ID, a.FrontMatter)
		if err := d.ReplaceLinks(row.ID, links); err != nil {
			return err
		}
		stats.Links += len(links)
		return nil
	})
	if walkErr != nil {
		return ReindexStats{}, fmt.Errorf("walk: %w", walkErr)
	}
	if err := d.SetLastReindex(time.Now()); err != nil {
		return ReindexStats{}, err
	}
	stats.DurationMS = time.Since(start).Milliseconds()
	return stats, nil
}
