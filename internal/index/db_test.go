package index

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".anvil", "vault.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

	for _, table := range []string{"artifacts", "links", "meta"} {
		var name string
		row := db.sql.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table)
		if err := row.Scan(&name); err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".anvil", "vault.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close() //nolint:errcheck,gosec // close in defer; error not actionable

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db2.Close() //nolint:errcheck,gosec // close in defer; error not actionable
}

// TestOpenMigratesStaleRuntimeColumn reproduces a vault.db created by an
// older anvil: build_tasks exists but predates transcript_path. Open must
// backfill the column rather than leaving inserts to hard-fail.
func TestOpenMigratesStaleRuntimeColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".anvil", "vault.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.sql.Exec(`ALTER TABLE build_tasks DROP COLUMN transcript_path`); err != nil {
		t.Fatalf("drop transcript_path: %v", err)
	}
	db.Close() //nolint:errcheck,gosec // close in defer; error not actionable

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close() //nolint:errcheck // close in defer; error not actionable

	var n int
	row := db2.sql.QueryRow(`SELECT count(*) FROM pragma_table_info('build_tasks') WHERE name = 'transcript_path'`)
	if err := row.Scan(&n); err != nil {
		t.Fatalf("check column: %v", err)
	}
	if n != 1 {
		t.Fatalf("transcript_path column not restored on reopen, got count=%d", n)
	}

	if _, err := db2.sql.Exec(
		`INSERT INTO build_tasks (run_id, task_id, wave, model, outcome) VALUES ('r', 't', 0, 'm', 'ok')`,
	); err != nil {
		t.Fatalf("insert after migration: %v", err)
	}
}
