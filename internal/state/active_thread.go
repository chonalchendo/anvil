// Package state owns small files under ~/.anvil/state/ that survive across CLI
// invocations. The first inhabitant is active-thread, consumed by skills and
// (eventually) the session emitter to bind sessions to a thread.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const activeThreadFile = "active-thread"

// ReadActiveThread returns the current active thread ID from <stateDir>/active-thread.
// A missing file is not an error — it means no thread is active.
func ReadActiveThread() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", fmt.Errorf("state dir: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, activeThreadFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read active-thread: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteActiveThread writes id as the active thread ID to <stateDir>/active-thread,
// creating the state directory if it does not exist.
func WriteActiveThread(id string) error {
	dir, err := stateDir()
	if err != nil {
		return fmt.Errorf("state dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, activeThreadFile), []byte(id+"\n"), 0o644); err != nil {
		return fmt.Errorf("write active-thread: %w", err)
	}
	return nil
}

// ClearActiveThread removes <stateDir>/active-thread.
// A missing file is not an error.
func ClearActiveThread() error {
	dir, err := stateDir()
	if err != nil {
		return fmt.Errorf("state dir: %w", err)
	}
	err = os.Remove(filepath.Join(dir, activeThreadFile))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove active-thread: %w", err)
}

// stateDir returns the state directory: ANVIL_STATE_DIR env if set, else ~/.anvil/state.
func stateDir() (string, error) {
	if d := os.Getenv("ANVIL_STATE_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".anvil", "state"), nil
}
