package index

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
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

	// A schema bump (e.g. a newly added derived table) needs one full rebuild to
	// backfill rows the incremental walk can't reconstruct from unchanged files.
	sv, err := d.GetSchemaVersion()
	if err != nil {
		return ReindexStats{}, err
	}
	if sv < SchemaVersion {
		return d.ReindexFull(vaultRoot)
	}

	// Capture the start BEFORE the walk and stamp THAT value (not a post-pass
	// time.Now()): any file edited during the pass must re-qualify next run.
	// A post-pass stamp would mark such an edit as ModTime ≤ stamp and skip it.
	start := time.Now()

	indexed, err := d.allArtifactPaths()
	if err != nil {
		return ReindexStats{}, err
	}
	storedPaths := make(map[string]struct{}, len(indexed))
	for _, path := range indexed {
		storedPaths[path] = struct{}{}
	}

	// Walk vault: collect current paths and the re-extract set. A file qualifies
	// when its mtime is newer than the stamp (edited) OR its path is not a known
	// stored path (added, or renamed-with-preserved-mtime — mv keeps mtime, so
	// the new path is otherwise ModTime ≤ stamp and would be missed).
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
		_, known := storedPaths[path]
		if info.ModTime().After(stamp) || !known {
			changed = append(changed, path)
		}
		return nil
	})
	if walkErr != nil {
		return ReindexStats{}, fmt.Errorf("walk: %w", walkErr)
	}

	// Re-extract changed/added/renamed files FIRST. A rename re-extracts the new
	// path, upserting the (path-independent) id onto its new path; the stale
	// old-path row is then purged below by the on-disk path check.
	for _, path := range changed {
		a, err := core.LoadArtifact(path)
		if err != nil {
			d.purgeStaleRowFor(path, indexed)
			continue
		}
		row, err := ArtifactRowFromFrontmatter(a.FrontMatter, path)
		if err != nil {
			d.purgeStaleRowFor(path, indexed)
			continue
		}
		// Duplicate-id collision: this changed file's id is already indexed at a
		// different path that is STILL on disk. The artifacts table is keyed by
		// id and holds one path per id, so the incremental walk can never see
		// both files as "known" — it re-extracts whichever colliding path isn't
		// currently stored and flips the row every pass, diverging from full's
		// deterministic last-writer-wins. A full rebuild is the only resolution
		// that is byte-identical to full and stable across runs. (Distinct from
		// a rename, where the prior path is absent from disk; handled by upsert.)
		if prior, ok := indexed[row.ID]; ok && prior != path {
			if _, stillOnDisk := onDisk[prior]; stillOnDisk {
				slog.Warn("duplicate artifact id across files; falling back to full reindex",
					"id", row.ID, "paths", []string{prior, path})
				return d.ReindexFull(vaultRoot)
			}
		}
		if err := d.UpsertArtifact(row); err != nil {
			return ReindexStats{}, err
		}
		links := append(LinkRowsFromFrontmatter(row.ID, a.FrontMatter), LinkRowsFromBody(row.ID, a.Body)...)
		if err := d.ReplaceLinks(row.ID, links); err != nil {
			return ReindexStats{}, err
		}
		if err := d.indexLearningFTS(row, a.Body); err != nil {
			return ReindexStats{}, err
		}
	}

	// Purge artifacts whose CURRENT stored path is absent from disk. Re-querying
	// after the re-extract above means a renamed id (now pointing at its new,
	// on-disk path) is not purged, while a genuine deletion (or a rename's stale
	// old-path row) is.
	current, err := d.allArtifactPaths()
	if err != nil {
		return ReindexStats{}, err
	}
	for id, path := range current {
		if _, ok := onDisk[path]; !ok {
			if derr := d.DeleteArtifact(id); derr != nil {
				return ReindexStats{}, fmt.Errorf("delete stale artifact %s: %w", id, derr)
			}
		}
	}

	if err := d.SetLastReindex(start); err != nil {
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
	if _, err := tx.Exec(`DELETE FROM learning_fts`); err != nil {
		return ReindexStats{}, fmt.Errorf("clear fts: %w", err)
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
		if err := d.indexLearningFTS(row, a.Body); err != nil {
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
	if err := d.SetSchemaVersion(SchemaVersion); err != nil {
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

// indexLearningFTS upserts a learning's TL;DR into the FTS table; a no-op for
// every other type. Called after each artifact upsert so content search stays
// in lockstep with the artifacts table.
func (d *DB) indexLearningFTS(row ArtifactRow, body string) error {
	if row.Type != string(core.TypeLearning) {
		return nil
	}
	return d.ReplaceLearningFTS(row.ID, LearningTLDR(body))
}

// purgeStaleRowFor drops the indexed row whose stored path equals path, used
// when a previously-valid file is edited into unparseable/malformed frontmatter.
// A full rebuild silently omits such a file; incremental must match by dropping
// the prior row rather than leaving it stale (the file stays on disk, so the
// deleted-file purge would never reach it). No-op if path was never indexed
// (a brand-new file that is unparseable was never a valid row).
func (d *DB) purgeStaleRowFor(path string, indexed map[string]string) {
	for id, p := range indexed {
		if p == path {
			// Best-effort: a delete failure here surfaces on the next full
			// rebuild; it does not corrupt the index.
			_ = d.DeleteArtifact(id) //nolint:errcheck // best-effort stale-row purge; see comment
			return
		}
	}
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
