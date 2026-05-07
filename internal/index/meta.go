package index

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// CheckFreshness returns ErrIndexStale if any .md file under vaultRoot is
// newer than the stored last-reindex stamp. ErrLastReindexUnset on first run.
//
// Why walk the tree: in-place edits to existing files don't bump the parent
// directory's mtime on macOS APFS or Linux ext4 — only structural changes
// (create/delete/rename) do. A vault-root stat alone misses an external
// editor saving over an existing artifact, which is exactly the kind of
// drift this check exists to surface.
func (d *DB) CheckFreshness(vaultRoot string) error {
	return d.CheckFreshnessExcept(vaultRoot, "")
}

// CheckFreshnessExcept is CheckFreshness but skips a single file path during
// the walk. The write-through hook calls this with the artifact it just
// saved, so the post-save freshness check doesn't trip on its own write
// while still catching drift in any other file.
func (d *DB) CheckFreshnessExcept(vaultRoot, skipPath string) error {
	stamp, err := d.GetLastReindex()
	if err != nil {
		return err
	}
	skipAbs := ""
	if skipPath != "" {
		if abs, aerr := filepath.Abs(skipPath); aerr == nil {
			skipAbs = abs
		}
	}
	walkErr := filepath.WalkDir(vaultRoot, func(path string, dEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dEntry.IsDir() {
			// Mirror reindex.go's skip rules: ignore .anvil and any other
			// dot-prefixed dir except vaultRoot itself.
			if dEntry.Name() == ".anvil" || (strings.HasPrefix(dEntry.Name(), ".") && path != vaultRoot) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		if skipAbs != "" {
			if abs, aerr := filepath.Abs(path); aerr == nil && abs == skipAbs {
				return nil
			}
		}
		info, ierr := dEntry.Info()
		if ierr != nil {
			return ierr
		}
		if info.ModTime().After(stamp) {
			return errStaleSentinel
		}
		return nil
	})
	if errors.Is(walkErr, errStaleSentinel) {
		return ErrIndexStale
	}
	if walkErr != nil {
		return fmt.Errorf("walk vault: %w", walkErr)
	}
	// Also catch deletes: vault dir mtime advances on a removed file even
	// though the file itself is gone.
	info, err := os.Stat(vaultRoot)
	if err != nil {
		return fmt.Errorf("stat vault: %w", err)
	}
	if info.ModTime().After(stamp) {
		return ErrIndexStale
	}
	return nil
}

// errStaleSentinel short-circuits the walk on first stale file. Internal —
// not part of the package's public error vocabulary.
var errStaleSentinel = errors.New("stale .md found")
