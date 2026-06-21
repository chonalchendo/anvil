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
CREATE TABLE IF NOT EXISTS tags (
    artifact TEXT NOT NULL,
    tag      TEXT NOT NULL,
    PRIMARY KEY (artifact, tag)
);
CREATE INDEX IF NOT EXISTS tags_tag_idx ON tags(tag);
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS eval_runs (
    skill     TEXT NOT NULL,
    source    TEXT NOT NULL,
    ref       TEXT NOT NULL DEFAULT '',
    passed    INTEGER,
    failed    INTEGER,
    total     INTEGER,
    pass_rate REAL NOT NULL,
    date      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS eval_runs_skill_idx ON eval_runs(skill);
-- build_runs / build_tasks are runtime-inserted by anvil build (like eval_runs),
-- not extracted from vault markdown. No SchemaVersion bump: Open creates them via
-- IF NOT EXISTS and ReindexFull never touches them, so there is nothing to backfill.
CREATE TABLE IF NOT EXISTS build_runs (
    run_id     TEXT PRIMARY KEY,
    started_at TEXT NOT NULL,
    project    TEXT,
    milestone  TEXT,
    dry_run    INTEGER NOT NULL DEFAULT 0,
    tasks      INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS build_tasks (
    run_id        TEXT NOT NULL,
    task_id       TEXT NOT NULL,
    phase         TEXT NOT NULL DEFAULT 'complete',
    wave          INTEGER NOT NULL,
    model         TEXT NOT NULL,
    effort        TEXT,
    outcome       TEXT NOT NULL,
    tokens_in     INTEGER NOT NULL DEFAULT 0,
    tokens_out    INTEGER NOT NULL DEFAULT 0,
    cache_read    INTEGER NOT NULL DEFAULT 0,
    cache_write   INTEGER NOT NULL DEFAULT 0,
    cost_usd      REAL NOT NULL DEFAULT 0,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    agent_time_ms INTEGER NOT NULL DEFAULT 0,
    verify_exit   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (run_id, task_id, phase)
);
CREATE INDEX IF NOT EXISTS build_tasks_run_idx ON build_tasks(run_id);
CREATE VIRTUAL TABLE IF NOT EXISTS learning_fts USING fts5(id UNINDEXED, tldr);
CREATE VIRTUAL TABLE IF NOT EXISTS artifact_fts USING fts5(id UNINDEXED, content);
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
//
// busy_timeout(5000) makes SQLite retry for up to 5 s before returning
// SQLITE_BUSY, which is enough to serialise concurrent anvil invocations on
// one vault without any application-level retry loop.
// journal_mode=WAL lets readers proceed concurrently with the single writer,
// cutting the window during which writers block each other.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	s, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := s.Exec(schema); err != nil {
		s.Close() //nolint:errcheck,gosec // cleanup before returning the schema error; close error not actionable
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{sql: s, path: path}, nil
}

// Close closes the underlying *sql.DB.
func (d *DB) Close() error { return d.sql.Close() }
