package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// installTranslatedAgents writes translate(src) for each embedded agent to
// target/<name>.md. A byte-identical file is a no-op; a divergent one is
// refused unless force is true. Shared by the markdown-emitting targets so
// the clobber contract lives in one place.
func installTranslatedAgents(srcFS fs.FS, target, targetName string, force bool, translate func([]byte) (string, error)) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return false, fmt.Errorf("mkdir target %s: %w", target, err)
	}
	changed := false
	for _, name := range names {
		src, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		want, err := translate(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		switch {
		case err == nil && string(got) == want:
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --target %s --force` to redeploy", dst, targetName)
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, []byte(want), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			return false, fmt.Errorf("write %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// removeTranslatedAgents deletes target/<name>.md for each embedded agent
// whose on-disk content still matches translate(src); divergent or foreign
// files are left untouched.
func removeTranslatedAgents(srcFS fs.FS, target string, translate func([]byte) (string, error)) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	changed := false
	for _, name := range names {
		src, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		want, err := translate(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if string(got) != want {
			continue
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}
