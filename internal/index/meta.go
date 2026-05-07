package index

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrLastReindexUnset means SetLastReindex has not been called yet.
var ErrLastReindexUnset = errors.New("last reindex stamp unset")

// ErrIndexStale means the vault directory mtime is newer than the stored
// last-reindex stamp. Callers should run `anvil reindex` and retry.
var ErrIndexStale = errors.New("vault index stale")

const metaKeyLastReindex = "last_reindex"

// SetLastReindex stamps the last-full-reindex time.
func (d *DB) SetLastReindex(t time.Time) error {
	const q = `INSERT INTO meta(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`
	if _, err := d.sql.Exec(q, metaKeyLastReindex, t.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("set last reindex: %w", err)
	}
	return nil
}

// GetLastReindex returns the stamped time. ErrLastReindexUnset if missing.
func (d *DB) GetLastReindex() (time.Time, error) {
	row := d.sql.QueryRow(`SELECT value FROM meta WHERE key = ?`, metaKeyLastReindex)
	var s string
	if err := row.Scan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, ErrLastReindexUnset
		}
		return time.Time{}, fmt.Errorf("get last reindex: %w", err)
	}
	return time.Parse(time.RFC3339Nano, s)
}

// CheckFreshness returns ErrIndexStale if the vault dir mtime is newer
// than the stored last-reindex stamp. ErrLastReindexUnset on first run.
func (d *DB) CheckFreshness(vaultRoot string) error {
	stamp, err := d.GetLastReindex()
	if err != nil {
		return err
	}
	info, err := os.Stat(vaultRoot)
	if err != nil {
		return fmt.Errorf("stat vault: %w", err)
	}
	if info.ModTime().After(stamp) {
		return ErrIndexStale
	}
	return nil
}
