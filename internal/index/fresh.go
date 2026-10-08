package index

import (
	"errors"
	"fmt"
	"log/slog"
)

// EnsureFresh reindexes when the vault drifted from the index: a bootstrap on
// first use, a WARN naming the drifted path, then a reindex on staleness. It
// is the one freshness policy every read path shares.
func (d *DB) EnsureFresh(root string) error {
	err := d.CheckFreshness(root)
	if err == nil {
		return nil
	}
	var stale *StaleError
	switch {
	case errors.Is(err, ErrLastReindexUnset):
		if _, err := d.Reindex(root); err != nil {
			return fmt.Errorf("bootstrap reindex: %w", err)
		}
	case errors.As(err, &stale):
		slog.Warn("vault index stale; auto-reindexing", "path", stale.Path, "reason", stale.Reason)
		if _, err := d.Reindex(root); err != nil {
			return fmt.Errorf("auto-reindex on stale: %w", err)
		}
	default:
		return fmt.Errorf("freshness check: %w", err)
	}
	return nil
}
