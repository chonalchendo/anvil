package index

import (
	"path/filepath"
	"sync"
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
	if _, err := db.sql.Exec(
		`INSERT INTO build_tasks (run_id, task_id, wave, model, outcome) VALUES ('r', 't', 0, 'm', 'ok')`,
	); err != nil {
		t.Fatalf("seed row before drop: %v", err)
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

	// The migration must backfill the column in place, not drop/recreate
	// the table — that would silently destroy the pre-existing row.
	var count int
	if err := db2.sql.QueryRow(`SELECT count(*) FROM build_tasks`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected pre-existing row to survive migration, got count=%d", count)
	}
	var transcriptPath string
	if err := db2.sql.QueryRow(
		`SELECT transcript_path FROM build_tasks WHERE run_id = 'r' AND task_id = 't'`,
	).Scan(&transcriptPath); err != nil {
		t.Fatalf("read backfilled row: %v", err)
	}
	if transcriptPath != "" {
		t.Fatalf("expected backfilled transcript_path to default to empty string, got %q", transcriptPath)
	}

	if _, err := db2.sql.Exec(
		`INSERT INTO build_tasks (run_id, task_id, wave, model, outcome) VALUES ('r2', 't2', 0, 'm', 'ok')`,
	); err != nil {
		t.Fatalf("insert after migration: %v", err)
	}
}

// TestOpenConcurrentMigratesStaleColumn reproduces racing anvil processes
// opening the same stale vault.db at once: only one Open should win the
// ALTER TABLE, and every other Open must still succeed rather than dying on
// "duplicate column name".
func TestOpenConcurrentMigratesStaleColumn(t *testing.T) {
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

	const openers = 8
	errs := make(chan error, openers)
	var wg sync.WaitGroup
	for i := 0; i < openers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := Open(path)
			if err != nil {
				errs <- err
				return
			}
			errs <- d.Close()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Open: %v", err)
		}
	}
}
