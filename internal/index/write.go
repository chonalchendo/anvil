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
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM links WHERE source = ?`, id); err != nil {
		return fmt.Errorf("delete links from %s: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM artifacts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete artifact %s: %w", id, err)
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
	defer tx.Rollback()
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
