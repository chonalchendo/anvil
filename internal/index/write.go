package index

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrArtifactNotInIndex is returned by GetArtifact when the row is absent.
var ErrArtifactNotInIndex = errors.New("artifact not in index")

// UpsertArtifact inserts or updates an artifact row. ON CONFLICT replaces all columns.
func (d *DB) UpsertArtifact(r ArtifactRow) error {
	const q = `INSERT INTO artifacts(id, type, status, project, path, created, updated)
VALUES(?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    type=excluded.type, status=excluded.status, project=excluded.project,
    path=excluded.path, created=excluded.created, updated=excluded.updated`
	if _, err := d.sql.Exec(q, r.ID, r.Type, r.Status, r.Project, r.Path, r.Created, r.Updated); err != nil {
		return fmt.Errorf("upsert artifact %s: %w", r.ID, err)
	}
	return nil
}

// GetArtifact reads a single artifact row by id.
func (d *DB) GetArtifact(id string) (ArtifactRow, error) {
	const q = `SELECT id, type, status, project, path, created, updated FROM artifacts WHERE id = ?`
	row := d.sql.QueryRow(q, id)
	var r ArtifactRow
	if err := row.Scan(&r.ID, &r.Type, &r.Status, &r.Project, &r.Path, &r.Created, &r.Updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ArtifactRow{}, ErrArtifactNotInIndex
		}
		return ArtifactRow{}, fmt.Errorf("get artifact %s: %w", id, err)
	}
	return r, nil
}

// DeleteArtifact removes the artifact and all its outgoing links. Idempotent.
func (d *DB) DeleteArtifact(id string) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit returns ErrTxDone; error not actionable
	if _, err := tx.Exec(`DELETE FROM links WHERE source = ?`, id); err != nil {
		return fmt.Errorf("delete links from %s: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM learning_fts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete learning fts for %s: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM artifact_fts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete artifact fts for %s: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM tags WHERE artifact = ?`, id); err != nil {
		return fmt.Errorf("delete tags for %s: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM artifacts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete artifact %s: %w", id, err)
	}
	return tx.Commit()
}

// ReplaceLearningFTS replaces the FTS row for a learning id: it drops any prior
// row and inserts the new TL;DR. An empty tldr clears the row without inserting
// (a learning whose TL;DR section is absent contributes nothing to search).
func (d *DB) ReplaceLearningFTS(id, tldr string) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit returns ErrTxDone; error not actionable
	if _, err := tx.Exec(`DELETE FROM learning_fts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("clear fts %s: %w", id, err)
	}
	if tldr != "" {
		if _, err := tx.Exec(`INSERT INTO learning_fts(id, tldr) VALUES(?, ?)`, id, tldr); err != nil {
			return fmt.Errorf("insert fts %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// ReplaceArtifactFTS replaces the FTS row for an issue or milestone: it drops
// any prior row and inserts the new content. An empty content string clears the
// row without inserting (artifact contributes nothing to content search).
func (d *DB) ReplaceArtifactFTS(id, content string) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit returns ErrTxDone; error not actionable
	if _, err := tx.Exec(`DELETE FROM artifact_fts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("clear artifact fts %s: %w", id, err)
	}
	if content != "" {
		if _, err := tx.Exec(`INSERT INTO artifact_fts(id, content) VALUES(?, ?)`, id, content); err != nil {
			return fmt.Errorf("insert artifact fts %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// ReplaceTags atomically replaces all facet rows for an artifact. Empty tags
// is allowed (clears the artifact's facets). Duplicate tags collapse on the
// (artifact, tag) primary key, so the caller need not pre-dedupe.
func (d *DB) ReplaceTags(artifact string, tags []string) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit returns ErrTxDone; error not actionable
	if _, err := tx.Exec(`DELETE FROM tags WHERE artifact = ?`, artifact); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	const ins = `INSERT OR IGNORE INTO tags(artifact, tag) VALUES(?, ?)`
	for _, t := range tags {
		if _, err := tx.Exec(ins, artifact, t); err != nil {
			return fmt.Errorf("insert tag %s/%s: %w", artifact, t, err)
		}
	}
	return tx.Commit()
}

// ReplaceLinks atomically replaces all outgoing edges from source. Empty
// rows is allowed (clears the source's edges).
func (d *DB) ReplaceLinks(source string, rows []LinkRow) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit returns ErrTxDone; error not actionable
	if _, err := tx.Exec(`DELETE FROM links WHERE source = ?`, source); err != nil {
		return fmt.Errorf("clear links: %w", err)
	}
	const ins = `INSERT INTO links(source, target, relation, anchor) VALUES(?, ?, ?, ?)`
	for _, r := range rows {
		if _, err := tx.Exec(ins, r.Source, r.Target, r.Relation, r.Anchor); err != nil {
			return fmt.Errorf("insert link %s→%s: %w", r.Source, r.Target, err)
		}
	}
	return tx.Commit()
}
