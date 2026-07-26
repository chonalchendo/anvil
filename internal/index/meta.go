package index

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrLastReindexUnset means SetLastReindex has not been called yet.
var ErrLastReindexUnset = errors.New("last reindex stamp unset")

// ErrIndexStale means the vault has drifted from the index: either a .md file
// or the vault directory itself has an mtime newer than the stored
// last-reindex stamp. Errors wrapping it name the offending path. Callers
// should run `anvil reindex` and retry.
var ErrIndexStale = errors.New("vault index stale")

const metaKeyLastReindex = "last_reindex"

const metaKeySchemaVersion = "schema_version"

// SchemaVersion bumps whenever a schema change needs a full rebuild to backfill
// derived rows from existing files (a new table the incremental walk can't
// populate from unchanged files). Reindex forces one full rebuild when the
// stored version lags — the no-migration "schema changed → rebuild" path.
//
//	1: learning_fts (FTS5 over learning TL;DR)
//	2: artifact_fts (FTS5 over issue/milestone description+goal for content-aware dedup)
//	3: tags (facet rows for `anvil index` relatedness)
//	4: design-type per-type flat folders (product-design → 05-product-designs/product-design.<project>.md, system-design → 06-system-designs/system-design.<project>[.<shard>].md; ids keep the type prefix for global artifacts.id uniqueness)
const SchemaVersion = 4

// GetSchemaVersion returns the stored schema version, or 0 when unset (a DB
// built before versioning, or a fresh one).
func (d *DB) GetSchemaVersion() (int, error) {
	row := d.sql.QueryRow(`SELECT value FROM meta WHERE key = ?`, metaKeySchemaVersion)
	var s string
	if err := row.Scan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get schema version: %w", err)
	}
	return strconv.Atoi(s)
}

// SetSchemaVersion stamps the schema version reached by a full rebuild.
func (d *DB) SetSchemaVersion(v int) error {
	const q = `INSERT INTO meta(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`
	if _, err := d.sql.Exec(q, metaKeySchemaVersion, strconv.Itoa(v)); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

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
//
// File modification times ahead of the clock are ignored — see skipFutureMtime.
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
	// One clock read for the whole walk, so files aren't compared against
	// drifting nows.
	now := time.Now()
	stalePath := ""
	onDisk := make(map[string]struct{})
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
		abs, aerr := filepath.Abs(path)
		if aerr == nil {
			onDisk[abs] = struct{}{}
		}
		if skipAbs != "" && abs == skipAbs {
			return nil
		}
		info, ierr := dEntry.Info()
		if ierr != nil {
			return ierr
		}
		if skipFutureMtime(path, info.ModTime(), now) {
			return nil
		}
		if info.ModTime().After(stamp) {
			stalePath = path
			return errStaleSentinel
		}
		return nil
	})
	if errors.Is(walkErr, errStaleSentinel) {
		return fmt.Errorf("%w: %s modified after last reindex (%s)", ErrIndexStale, stalePath, stamp.Format(time.RFC3339))
	}
	if walkErr != nil {
		return fmt.Errorf("walk vault: %w", walkErr)
	}
	// Compare the indexed file set against what's actually on disk, so a
	// deleted file is caught directly rather than inferred from the vault
	// directory's mtime advancing.
	if deleted, derr := d.firstDeletedArtifactPath(onDisk, skipAbs); derr != nil {
		return derr
	} else if deleted != "" {
		return fmt.Errorf("%w: %s no longer exists on disk (last reindex %s)", ErrIndexStale, deleted, stamp.Format(time.RFC3339))
	}
	// Also catch other structural drift (e.g. an add/rename this pass didn't
	// otherwise see): vault dir mtime advances on any such change. This arm
	// intentionally honours future mtimes: callers signal external drift by
	// stamping the root a moment ahead of now (see markVaultExternallyStale),
	// so guarding it would suppress drift absorption.
	info, err := os.Stat(vaultRoot)
	if err != nil {
		return fmt.Errorf("stat vault: %w", err)
	}
	if info.ModTime().After(stamp) {
		return fmt.Errorf("%w: %s changed after last reindex (%s)", ErrIndexStale, vaultRoot, stamp.Format(time.RFC3339))
	}
	return nil
}

// firstDeletedArtifactPath returns the first indexed artifact path that is no
// longer present on disk (deterministic ordering by id), or "" if every
// indexed path still exists. skipAbs excludes the file the caller just wrote
// (already known to exist; a stale row for it would be a race, not a delete).
func (d *DB) firstDeletedArtifactPath(onDisk map[string]struct{}, skipAbs string) (string, error) {
	indexed, err := d.allArtifactPaths()
	if err != nil {
		return "", fmt.Errorf("check deleted artifacts: %w", err)
	}
	ids := make([]string, 0, len(indexed))
	for id := range indexed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		path := indexed[id]
		abs, aerr := filepath.Abs(path)
		if aerr != nil {
			abs = path
		}
		if abs == skipAbs {
			continue
		}
		if _, ok := onDisk[abs]; !ok {
			return path, nil
		}
	}
	return "", nil
}

// skipFutureMtime reports whether a walked file's mtime post-dates the check's
// clock read, and warns when it does. Such a timestamp can't record an edit
// that has already happened; more importantly it is newer than any stamp
// `anvil reindex` can write (reindex stamps time.Now()), so counting it as
// drift would leave the index stale forever with no operator-reachable fix.
// It guards the file walk only — never the vault-root stat, whose future-mtime
// behaviour is load-bearing for drift absorption.
func skipFutureMtime(path string, mtime, now time.Time) bool {
	if !mtime.After(now) {
		return false
	}
	slog.Warn("ignoring future modification time in freshness check",
		"path", path, "mtime", mtime.Format(time.RFC3339), "now", now.Format(time.RFC3339))
	return true
}

// errStaleSentinel short-circuits the walk on first stale file. Internal —
// not part of the package's public error vocabulary.
var errStaleSentinel = errors.New("stale .md found")
