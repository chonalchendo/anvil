package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// skillsHashFile marks the materialised skills tree with the hash of the
// embedded source FS so future invocations can detect drift.
const skillsHashFile = ".anvil-skills-hash"

// skillsDeployStampFile records the binary's mtime at install time so the
// auto-refresh path can detect whether the current binary was rebuilt after
// the last install and thus has the right to overwrite the installed bundle.
const skillsDeployStampFile = ".anvil-skills-deploy-stamp"

// binaryMtime returns the modification time of the running executable.
// Overridable in tests to simulate different binary ages without filesystem
// manipulation.
var binaryMtime = func() (time.Time, error) {
	exe, err := os.Executable()
	if err != nil {
		return time.Time{}, fmt.Errorf("executable: %w", err)
	}
	info, err := os.Stat(exe)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat executable: %w", err)
	}
	return info.ModTime(), nil
}

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
// When force is true, foreign non-symlink dirs and non-anvil dirs at
// target/<name> are deleted and replaced — matching the --force flag's
// promise to the user. When false, those paths are refused with a hint
// that names --force as the next command.
//
// Returns changed=false only when every per-skill entry is already in place
// and no legacy artefact was removed.
func InstallSkills(srcFS fs.FS, materialiseDir, target string, useCopy, force bool) (bool, error) {
	hash, err := computeSkillsHash(srcFS)
	if err != nil {
		return false, fmt.Errorf("hash skills: %w", err)
	}
	if err := writeFSTree(srcFS, materialiseDir); err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(materialiseDir, skillsHashFile), []byte(hash), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		return false, fmt.Errorf("write skills hash: %w", err)
	}
	// Record the deploying binary's mtime as a provenance stamp so auto-refresh
	// can distinguish "this binary was rebuilt" (stamp < binary mtime → refresh
	// safe) from "a different binary deployed" (stamp >= binary mtime → skip).
	// Failure to record the stamp is non-fatal; auto-refresh falls back to the
	// legacy hash-only check on the next invocation.
	if btime, berr := binaryMtime(); berr == nil {
		stamp := strconv.FormatInt(btime.UnixNano(), 10)
		_ = os.WriteFile(filepath.Join(materialiseDir, skillsDeployStampFile), []byte(stamp), 0o644) //nolint:gosec // 0644 is correct for config/data files readable by owner and group
	}
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
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
		c, err := installOneSkill(materialiseDir, target, name, useCopy, force)
		if err != nil {
			return false, err
		}
		if c {
			changed = true
		}
	}
	pruned, err := pruneOrphanedSkills(materialiseDir, target, names)
	if err != nil {
		return false, err
	}
	if pruned {
		changed = true
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
//
// If a deploy stamp is present (written by InstallSkills), the refresh only
// proceeds when the current binary is strictly newer than the stamp — meaning
// the binary was rebuilt since the last install. When the binary is the same
// age or older than the stamp, the installed bundle came from a newer or
// different binary and must not be overwritten. The stamp check is skipped
// (falling back to hash-only logic) when the stamp file is absent, to
// preserve backward compatibility with installs done before this guard was
// introduced.
//
// If any target/<name> entry would block install (user planted a regular
// dir over what should be a symlink, or a non-anvil dir in copy mode), the
// refresh silently no-ops without touching materialiseDir. The implicit
// auto-refresh path must not flood stderr when the user has chosen to
// manage that skill path manually; the explicit `anvil install skills`
// command, which calls InstallSkills directly, still surfaces the refusal
// and points at --force — that is where the user expects to be told.
// Leaving materialiseDir untouched also preserves the stale-hash signal so
// the explicit install path continues to detect the conflict instead of
// short-circuiting on freshness.
func RefreshSkillsIfStale(srcFS fs.FS, materialiseDir, target string) (bool, error) {
	if _, err := os.Stat(materialiseDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", materialiseDir, err)
	}

	// Version guard: when a deploy stamp is present, only refresh if the
	// current binary is strictly newer than the stamp. A stamp at or after the
	// binary's mtime means a different (or equally-aged) binary last deployed —
	// skip to avoid overwriting a divergent bundle.
	if stampBytes, err := os.ReadFile(filepath.Join(materialiseDir, skillsDeployStampFile)); err == nil { //nolint:gosec // path is application-managed
		if btime, berr := binaryMtime(); berr == nil {
			if stampNanos, perr := strconv.ParseInt(strings.TrimSpace(string(stampBytes)), 10, 64); perr == nil {
				if !btime.After(time.Unix(0, stampNanos)) {
					return false, nil
				}
			}
		}
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
	blocked, err := refreshBlocked(srcFS, target, useCopy)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, nil
	}
	if _, err := InstallSkills(srcFS, materialiseDir, target, useCopy, false); err != nil {
		return false, err
	}
	return true, nil
}

// refreshBlocked reports whether any target/<name> entry would cause
// InstallSkills(..., force=false) to refuse: a regular dir at a symlink
// path in symlink mode, or a non-anvil dir in copy mode. The check is
// per-skill, mirroring installOneSkill's own logic — keeping them in sync
// is cheap because both only inspect target/<name>.
func refreshBlocked(srcFS fs.FS, target string, useCopy bool) (bool, error) {
	names, err := listSkillNames(srcFS)
	if err != nil {
		return false, err
	}
	for _, name := range names {
		dst := filepath.Join(target, name)
		info, err := os.Lstat(dst)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("stat %s: %w", dst, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !useCopy {
			return true, nil
		}
		// Copy mode: a directory without the skill marker is user-owned.
		_, statErr := os.Stat(filepath.Join(dst, skillMarker))
		if errors.Is(statErr, os.ErrNotExist) {
			return true, nil
		}
		if statErr != nil {
			return false, fmt.Errorf("stat skill marker %s: %w", dst, statErr)
		}
	}
	return false, nil
}

// SkillsAreFresh reports whether the hash recorded under materialiseDir
// matches srcFS. Returns false when the hash file is missing — callers
// treat that as drift so a missing marker forces a refresh.
func SkillsAreFresh(srcFS fs.FS, materialiseDir string) (bool, error) {
	expected, err := computeSkillsHash(srcFS)
	if err != nil {
		return false, fmt.Errorf("hash skills: %w", err)
	}
	onDisk, err := os.ReadFile(filepath.Join(materialiseDir, skillsHashFile)) //nolint:gosec // path is test-controlled or application-managed; not user input
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

func installOneSkill(materialiseDir, target, name string, useCopy, force bool) (bool, error) {
	dst := filepath.Join(target, name)
	src := filepath.Join(materialiseDir, name)
	if useCopy {
		return installSkillCopy(src, dst, force)
	}
	return installSkillSymlink(src, dst, force)
}

func installSkillSymlink(src, dst string, force bool) (bool, error) {
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
		if !force {
			return false, fmt.Errorf("refusing to overwrite non-symlink %s; run `anvil install skills --force` to redeploy, or `rm -rf %q && anvil install skills` to take the destructive path", dst, dst)
		}
		if err := os.RemoveAll(dst); err != nil {
			return false, fmt.Errorf("clear %s: %w", dst, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("stat %s: %w", dst, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return false, fmt.Errorf("symlink %s -> %s: %w", dst, src, err)
	}
	return true, nil
}

func installSkillCopy(src, dst string, force bool) (bool, error) {
	info, err := os.Lstat(dst)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("replace symlink %s: %w", dst, err)
		}
	case err == nil:
		if _, mErr := os.Stat(filepath.Join(dst, skillMarker)); mErr != nil && !force {
			return false, fmt.Errorf("refusing to overwrite non-anvil dir %s; run `anvil install skills --force` to redeploy, or `rm -rf %q && anvil install skills` to take the destructive path", dst, dst)
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
	if err := os.WriteFile(filepath.Join(dst, skillMarker), nil, 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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

// PruneOrphanedSkills removes anvil-owned symlinks in target that are absent
// from the current bundle in srcFS. Foreign entries and non-anvil-owned
// symlinks are never touched. It is safe to call when InstallSkills was
// skipped (e.g. bundle hash is fresh) — the prune reconciles target to match
// the current bundle regardless.
func PruneOrphanedSkills(srcFS fs.FS, materialiseDir, target string) (bool, error) {
	names, err := listSkillNames(srcFS)
	if err != nil {
		return false, err
	}
	return pruneOrphanedSkills(materialiseDir, target, names)
}

// pruneOrphanedSkills removes anvil-owned symlinks in target that are absent
// from the current bundle (names). Foreign entries and non-anvil-owned symlinks
// are never touched.
func pruneOrphanedSkills(materialiseDir, target string, names []string) (bool, error) {
	entries, err := os.ReadDir(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read target dir %s: %w", target, err)
	}
	current := make(map[string]struct{}, len(names))
	for _, n := range names {
		current[n] = struct{}{}
	}
	changed := false
	for _, e := range entries {
		if _, ok := current[e.Name()]; ok {
			continue
		}
		if e.Type()&os.ModeSymlink == 0 {
			continue
		}
		linkTarget, err := os.Readlink(filepath.Join(target, e.Name()))
		if err != nil {
			return false, fmt.Errorf("readlink %s: %w", e.Name(), err)
		}
		if !ownsSymlinkTarget(linkTarget, materialiseDir) {
			continue
		}
		if err := os.Remove(filepath.Join(target, e.Name())); err != nil {
			return false, fmt.Errorf("remove orphaned symlink %s: %w", e.Name(), err)
		}
		changed = true
	}
	return changed, nil
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
	if err := os.MkdirAll(dst, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
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
			return os.MkdirAll(out, 0o755) //nolint:gosec // 0755 is correct for directories that must be traversable
		}
		data, err := fs.ReadFile(srcFS, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			return fmt.Errorf("write %s: %w", out, err)
		}
		return nil
	})
}
