package index

import (
	"errors"
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

// Reindex re-processes only files whose mtime is newer than the stored
// last_reindex stamp, leaving unchanged rows intact. Falls back to ReindexFull
// when the stamp is absent (first run). Stamps last_reindex on success.
func (d *DB) Reindex(vaultRoot string) (ReindexStats, error) {
	stamp, err := d.GetLastReindex()
	if errors.Is(err, ErrLastReindexUnset) {
		return d.ReindexFull(vaultRoot)
	}
	if err != nil {
		return ReindexStats{}, fmt.Errorf("get last reindex: %w", err)
	}

	start := time.Now()

	// Walk vault: collect current paths and detect changed/added files.
	var changed []string
	onDisk := make(map[string]struct{})

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
		onDisk[path] = struct{}{}
		info, ierr := dEntry.Info()
		if ierr != nil {
			return ierr
		}
		if info.ModTime().After(stamp) {
			changed = append(changed, path)
		}
		return nil
	})
	if walkErr != nil {
		return ReindexStats{}, fmt.Errorf("walk: %w", walkErr)
	}

	// Purge artifacts whose backing file has been deleted.
	indexed, err := d.allArtifactPaths()
	if err != nil {
		return ReindexStats{}, err
	}
	for id, path := range indexed {
		if _, ok := onDisk[path]; !ok {
			if derr := d.DeleteArtifact(id); derr != nil {
				return ReindexStats{}, fmt.Errorf("delete stale artifact %s: %w", id, derr)
			}
		}
	}

	// Re-extract changed/added files.
	for _, path := range changed {
		a, err := core.LoadArtifact(path)
		if err != nil {
			continue //nolint:nilerr // incremental is best-effort; unparseable files are skipped
		}
		row, err := ArtifactRowFromFrontmatter(a.FrontMatter, path)
		if err != nil {
			continue //nolint:nilerr // incremental is best-effort; malformed frontmatter is skipped
		}
		if err := d.UpsertArtifact(row); err != nil {
			return ReindexStats{}, err
		}
		links := append(LinkRowsFromFrontmatter(row.ID, a.FrontMatter), LinkRowsFromBody(row.ID, a.Body)...)
		if err := d.ReplaceLinks(row.ID, links); err != nil {
			return ReindexStats{}, err
		}
	}

	if err := d.SetLastReindex(time.Now()); err != nil {
		return ReindexStats{}, err
	}

	// Report totals from DB so the count is byte-identical to a full rebuild.
	artifacts, links, err := d.countArtifactsAndLinks()
	if err != nil {
		return ReindexStats{}, err
	}
	return ReindexStats{
		Artifacts:  artifacts,
		Links:      links,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// ReindexFull tears down both tables and walks the vault to repopulate them.
// Stamps the last-reindex time on success. Use Reindex for the common case.
func (d *DB) ReindexFull(vaultRoot string) (ReindexStats, error) {
	start := time.Now()
	tx, err := d.sql.Begin()
	if err != nil {
		return ReindexStats{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit returns ErrTxDone; error not actionable
	if _, err := tx.Exec(`DELETE FROM links`); err != nil {
		return ReindexStats{}, fmt.Errorf("clear links: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM artifacts`); err != nil {
		return ReindexStats{}, fmt.Errorf("clear artifacts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ReindexStats{}, fmt.Errorf("commit clear: %w", err)
	}

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
			return nil //nolint:nilerr // reindex is best-effort; unparseable files are skipped
		}
		row, err := ArtifactRowFromFrontmatter(a.FrontMatter, path)
		if err != nil {
			return nil //nolint:nilerr // reindex is best-effort; malformed frontmatter is skipped
		}
		if err := d.UpsertArtifact(row); err != nil {
			return err
		}
		links := append(LinkRowsFromFrontmatter(row.ID, a.FrontMatter), LinkRowsFromBody(row.ID, a.Body)...)
		if err := d.ReplaceLinks(row.ID, links); err != nil {
			return err
		}
		return nil
	})
	if walkErr != nil {
		return ReindexStats{}, fmt.Errorf("walk: %w", walkErr)
	}
	if err := d.SetLastReindex(time.Now()); err != nil {
		return ReindexStats{}, err
	}
	// Report from DB so duplicate IDs (same id in multiple files) don't
	// inflate the counter beyond the actual row count.
	artifacts, links, err := d.countArtifactsAndLinks()
	if err != nil {
		return ReindexStats{}, err
	}
	return ReindexStats{
		Artifacts:  artifacts,
		Links:      links,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// allArtifactPaths returns a map from artifact id to its stored file path.
func (d *DB) allArtifactPaths() (map[string]string, error) {
	rows, err := d.sql.Query(`SELECT id, path FROM artifacts`)
	if err != nil {
		return nil, fmt.Errorf("query artifact paths: %w", err)
	}
	defer rows.Close() //nolint:errcheck // close in defer; error not actionable
	m := make(map[string]string)
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, fmt.Errorf("scan artifact path: %w", err)
		}
		m[id] = path
	}
	return m, rows.Err()
}

// countArtifactsAndLinks returns the current row counts from both tables.
func (d *DB) countArtifactsAndLinks() (artifacts, links int, err error) {
	if err = d.sql.QueryRow(`SELECT COUNT(*) FROM artifacts`).Scan(&artifacts); err != nil {
		return 0, 0, fmt.Errorf("count artifacts: %w", err)
	}
	if err = d.sql.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&links); err != nil {
		return 0, 0, fmt.Errorf("count links: %w", err)
	}
	return artifacts, links, nil
}
