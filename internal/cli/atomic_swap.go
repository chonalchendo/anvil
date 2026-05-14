package cli

import (
	"fmt"
	"os"
)

// swapFault is the set of injectable failure points exercised by tests. Each
// field, when non-nil, causes atomicSwap to return the given error at the
// matching step. Tests assign to swapHook for a single call and clear it
// after; production callers never touch this.
type swapFault struct {
	afterTempWrite  error // fail before renaming old → bak
	afterOldToBak   error // fail between rename old→bak and rename tmp→new
	afterTempToNew  error // fail between rename tmp→new and remove bak
}

var swapHook *swapFault

// atomicSwap replaces oldPath with newPath holding content via a three-step
// rename swap with rollback on partial failure. Layout:
//
//  1. write content to <newPath>.tmp + fsync + close
//  2. rename(oldPath, <oldPath>.bak)
//  3. rename(<newPath>.tmp, newPath)
//  4. remove(<oldPath>.bak)
//
// Any SIGKILL between steps leaves at most one of
// {oldPath, <oldPath>.bak, <newPath>.tmp} alongside newPath — never a
// duplicate-content pair at oldPath and newPath simultaneously.
//
// Rollback on step 3 restores oldPath from <oldPath>.bak so callers see the
// pre-rename state. A failure on step 4 is non-fatal — the new state is
// correct; the leftover .bak is reported via the returned error so the caller
// can warn.
func atomicSwap(oldPath, newPath string, content []byte) error {
	tmpPath := newPath + ".tmp"
	bakPath := oldPath + ".bak"

	if err := writeFileSync(tmpPath, content, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if swapHook != nil && swapHook.afterTempWrite != nil {
		_ = os.Remove(tmpPath)
		return swapHook.afterTempWrite
	}

	if err := os.Rename(oldPath, bakPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming old to backup: %w", err)
	}
	if swapHook != nil && swapHook.afterOldToBak != nil {
		// Roll back: restore old from bak, drop tmp.
		_ = os.Rename(bakPath, oldPath)
		_ = os.Remove(tmpPath)
		return swapHook.afterOldToBak
	}

	if err := os.Rename(tmpPath, newPath); err != nil {
		// Roll back: restore oldPath from backup, drop tmp.
		if rerr := os.Rename(bakPath, oldPath); rerr != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("renaming temp to new: %w (rollback also failed: %v; backup left at %s)", err, rerr, bakPath)
		}
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp to new: %w", err)
	}
	if swapHook != nil && swapHook.afterTempToNew != nil {
		// New state is already correct on disk. Surface the injected error
		// without removing the backup so tests can observe both end states.
		return swapHook.afterTempToNew
	}

	if err := os.Remove(bakPath); err != nil {
		return fmt.Errorf("removing backup (rename succeeded; backup left at %s): %w", bakPath, err)
	}
	return nil
}

// writeFileSync writes data to path with fsync, ensuring durability before
// the subsequent rename. Mirrors os.WriteFile but adds the fsync step.
func writeFileSync(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	return f.Close()
}
