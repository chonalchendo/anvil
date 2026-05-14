package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// skillsHashFile marks the materialised skills tree with the hash of the
// embedded source FS so future invocations can detect drift.
const skillsHashFile = ".anvil-skills-hash"

// skillMarker is dropped inside each per-skill directory in --copy mode so
// uninstall and refresh can recognise anvil-owned content vs. user content
// that happens to share a skill name.
const skillMarker = ".anvil-skill"

// legacyNamespace is the subdirectory used by pre-flat-layout installs that
// nested every skill under <target>/anvil/<name>/. It is removed on install
// and uninstall when the directory is recognisably an old anvil install.
const legacyNamespace = "anvil"

// InstallSkills materialises srcFS to materialiseDir, then exposes every
// top-level skill at target/<name> as a symlink into materialiseDir/<name>
// (or a copied tree when useCopy is true). target is the user-skills parent
// directory (typically ~/.claude/skills), shared with non-anvil skills.
// A legacy target/anvil/ nested install is cleaned up if detected.
//
// Returns changed=false only when every per-skill entry is already in place
// and no legacy artefact was removed.
func InstallSkills(srcFS fs.FS, materialiseDir, target string, useCopy bool) (bool, error) {
	hash, err := computeSkillsHash(srcFS)
	if err != nil {
		return false, fmt.Errorf("hash skills: %w", err)
	}
	if err := writeFSTree(srcFS, materialiseDir); err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(materialiseDir, skillsHashFile), []byte(hash), 0o644); err != nil {
		return false, fmt.Errorf("write skills hash: %w", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false, fmt.Errorf("mkdir target %s: %w", target, err)
	}

	changed := false
	legacyRemoved, err := removeLegacyNamespace(target)
	if err != nil {
		return false, err
	}
	if legacyRemoved {
		changed = true
	}

	names, err := listSkillNames(srcFS)
	if err != nil {
		return false, err
	}
	for _, name := range names {
		c, err := installOneSkill(materialiseDir, target, name, useCopy)
		if err != nil {
			return false, err
		}
		if c {
			changed = true
		}
	}
	return changed, nil
}

// RemoveSkills removes target/<name> for each top-level skill name in srcFS.
// Foreign sibling entries in target are left untouched. Legacy target/anvil/
// nested installs are also cleaned up.
func RemoveSkills(srcFS fs.FS, materialiseDir, target string) (bool, error) {
	if _, err := os.Stat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", target, err)
	}
	names, err := listSkillNames(srcFS)
	if err != nil {
		return false, err
	}
	changed := false
	for _, name := range names {
		c, err := removeOneSkill(materialiseDir, target, name)
		if err != nil {
			return false, err
		}
		if c {
			changed = true
		}
	}
	legacyRemoved, err := removeLegacyNamespace(target)
	if err != nil {
		return false, err
	}
	if legacyRemoved {
		changed = true
	}
	return changed, nil
}

// RefreshSkillsIfStale rewrites the installed bundle when the hash recorded
// in materialiseDir diverges from srcFS. No-op when materialiseDir is absent.
// The install mode (symlink vs copy) is recovered by inspecting any existing
// per-skill child under target.
func RefreshSkillsIfStale(srcFS fs.FS, materialiseDir, target string) (bool, error) {
	if _, err := os.Stat(materialiseDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", materialiseDir, err)
	}
	fresh, err := SkillsAreFresh(srcFS, materialiseDir)
	if err != nil {
		return false, err
	}
	if fresh {
		return false, nil
	}
	useCopy, err := detectCopyMode(srcFS, target)
	if err != nil {
		return false, err
	}
	if _, err := InstallSkills(srcFS, materialiseDir, target, useCopy); err != nil {
		return false, err
	}
	return true, nil
}

// SkillsAreFresh reports whether the hash recorded under materialiseDir
// matches srcFS. Returns false when the hash file is missing — callers
// treat that as drift so a missing marker forces a refresh.
func SkillsAreFresh(srcFS fs.FS, materialiseDir string) (bool, error) {
	expected, err := computeSkillsHash(srcFS)
	if err != nil {
		return false, fmt.Errorf("hash skills: %w", err)
	}
	onDisk, err := os.ReadFile(filepath.Join(materialiseDir, skillsHashFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read skills hash: %w", err)
	}
	return strings.TrimSpace(string(onDisk)) == expected, nil
}

func listSkillNames(srcFS fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read skills FS: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

func installOneSkill(materialiseDir, target, name string, useCopy bool) (bool, error) {
	dst := filepath.Join(target, name)
	src := filepath.Join(materialiseDir, name)
	if useCopy {
		return installSkillCopy(src, dst)
	}
	return installSkillSymlink(src, dst)
}

func installSkillSymlink(src, dst string) (bool, error) {
	info, err := os.Lstat(dst)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		current, _ := os.Readlink(dst)
		if current == src {
			return false, nil
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("replace symlink %s: %w", dst, err)
		}
	case err == nil:
		return false, fmt.Errorf("refusing to overwrite non-symlink %s; run `anvil install skills --force` to redeploy, or `rm -rf %s && anvil install skills` to take the destructive path", dst, dst)
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("stat %s: %w", dst, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return false, fmt.Errorf("symlink %s -> %s: %w", dst, src, err)
	}
	return true, nil
}

func installSkillCopy(src, dst string) (bool, error) {
	info, err := os.Lstat(dst)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("replace symlink %s: %w", dst, err)
		}
	case err == nil:
		if _, mErr := os.Stat(filepath.Join(dst, skillMarker)); mErr != nil {
			return false, fmt.Errorf("refusing to overwrite non-anvil dir %s; run `anvil install skills --force` to redeploy, or `rm -rf %s && anvil install skills` to take the destructive path", dst, dst)
		}
		if err := os.RemoveAll(dst); err != nil {
			return false, fmt.Errorf("clear %s: %w", dst, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("stat %s: %w", dst, err)
	}
	if err := writeFSTree(os.DirFS(src), dst); err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(dst, skillMarker), nil, 0o644); err != nil {
		return false, fmt.Errorf("write skill marker: %w", err)
	}
	return true, nil
}

func removeOneSkill(materialiseDir, target, name string) (bool, error) {
	dst := filepath.Join(target, name)
	info, err := os.Lstat(dst)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", dst, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		current, _ := os.Readlink(dst)
		if !ownsSymlinkTarget(current, materialiseDir) {
			return false, nil
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove symlink %s: %w", dst, err)
		}
		return true, nil
	}
	if _, err := os.Stat(filepath.Join(dst, skillMarker)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat skill marker %s: %w", dst, err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return false, fmt.Errorf("remove %s: %w", dst, err)
	}
	return true, nil
}

func ownsSymlinkTarget(linkTarget, materialiseDir string) bool {
	if linkTarget == materialiseDir {
		return true
	}
	return strings.HasPrefix(linkTarget, materialiseDir+string(filepath.Separator))
}

func removeLegacyNamespace(target string) (bool, error) {
	legacy := filepath.Join(target, legacyNamespace)
	info, err := os.Lstat(legacy)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat legacy %s: %w", legacy, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(legacy); err != nil {
			return false, fmt.Errorf("remove legacy symlink %s: %w", legacy, err)
		}
		return true, nil
	}
	if _, err := os.Stat(filepath.Join(legacy, skillsHashFile)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat legacy hash %s: %w", legacy, err)
	}
	if err := os.RemoveAll(legacy); err != nil {
		return false, fmt.Errorf("remove legacy %s: %w", legacy, err)
	}
	return true, nil
}

func detectCopyMode(srcFS fs.FS, target string) (bool, error) {
	names, err := listSkillNames(srcFS)
	if err != nil {
		return false, err
	}
	for _, name := range names {
		child := filepath.Join(target, name)
		info, err := os.Lstat(child)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("stat %s: %w", child, err)
		}
		return info.Mode()&os.ModeSymlink == 0, nil
	}
	return false, nil
}

func computeSkillsHash(srcFS fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, rerr := fs.ReadFile(srcFS, p)
		if rerr != nil {
			return fmt.Errorf("read %s: %w", p, rerr)
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
