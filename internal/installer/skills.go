package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// InstallSkills materialises srcFS to materialiseDir, then wires target so
// that path resolves to those files. When useCopy is false (default), target
// is created as a symlink pointing at materialiseDir; when true, the
// materialised tree is copied to target instead (for filesystems that don't
// support symlinks). Existing target symlinks are replaced; existing target
// directories cause an error to avoid clobbering user content.
//
// Returns changed=false only when a symlink at target already points at
// materialiseDir and the materialised tree didn't need updating.
func InstallSkills(srcFS fs.FS, materialiseDir, target string, useCopy bool) (bool, error) {
	if err := writeFSTree(srcFS, materialiseDir); err != nil {
		return false, err
	}
	if useCopy {
		return installSkillsCopy(materialiseDir, target)
	}
	return installSkillsSymlink(materialiseDir, target)
}

// RemoveSkills removes target whether it is a symlink (the default install
// shape) or a directory (the --copy shape). It does not touch the
// materialised tree, which is binary-owned state. Returns changed=false when
// target was already absent.
func RemoveSkills(target string) (bool, error) {
	info, err := os.Lstat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", target, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("remove symlink %s: %w", target, err)
		}
		return true, nil
	}
	if err := os.RemoveAll(target); err != nil {
		return false, fmt.Errorf("remove %s: %w", target, err)
	}
	return true, nil
}

func installSkillsSymlink(materialiseDir, target string) (bool, error) {
	info, err := os.Lstat(target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		current, _ := os.Readlink(target)
		if current == materialiseDir {
			return false, nil
		}
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("replace symlink %s: %w", target, err)
		}
	case err == nil:
		return false, fmt.Errorf("refusing to overwrite non-symlink %s; remove it first or use --copy", target)
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("stat %s: %w", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, fmt.Errorf("mkdir parent of %s: %w", target, err)
	}
	if err := os.Symlink(materialiseDir, target); err != nil {
		return false, fmt.Errorf("symlink %s -> %s: %w", target, materialiseDir, err)
	}
	return true, nil
}

func installSkillsCopy(materialiseDir, target string) (bool, error) {
	info, err := os.Lstat(target)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("replace symlink %s: %w", target, err)
		}
	} else if err == nil {
		if err := os.RemoveAll(target); err != nil {
			return false, fmt.Errorf("clear %s: %w", target, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", target, err)
	}
	if err := writeFSTree(os.DirFS(materialiseDir), target); err != nil {
		return false, err
	}
	return true, nil
}

// writeFSTree extracts every file in srcFS to dst, clearing any prior
// contents of dst first so the result is a faithful mirror of srcFS.
func writeFSTree(srcFS fs.FS, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("clear %s: %w", dst, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dst, err)
	}
	return fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		out := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := fs.ReadFile(srcFS, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		return nil
	})
}
