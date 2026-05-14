package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the pure-Go sqlite driver with database/sql
)

const schema = `
CREATE TABLE IF NOT EXISTS artifacts (
    id       TEXT PRIMARY KEY,
    type     TEXT NOT NULL,
    status   TEXT,
    project  TEXT,
    path     TEXT NOT NULL,
    created  TEXT,
    updated  TEXT
);
CREATE TABLE IF NOT EXISTS links (
    source   TEXT NOT NULL,
    target   TEXT NOT NULL,
    relation TEXT NOT NULL,
    anchor   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (source, target, relation, anchor)
);
CREATE INDEX IF NOT EXISTS links_target_idx ON links(target);
CREATE INDEX IF NOT EXISTS artifacts_type_status_idx ON artifacts(type, status);
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

// DB wraps the sql.DB and owns its lifecycle.
type DB struct {
	sql  *sql.DB
	path string
}

// DBPath returns the canonical vault.db location for a given vault root.
func DBPath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".anvil", "vault.db")
}

// Open opens (or creates) the DB at path, ensuring the parent directory
// exists and the schema is applied. Idempotent.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	s, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := s.Exec(schema); err != nil {
		s.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{sql: s, path: path}, nil
}

// Close closes the underlying *sql.DB.
func (d *DB) Close() error { return d.sql.Close() }
