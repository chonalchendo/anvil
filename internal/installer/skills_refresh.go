package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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
// The stamp is the deploying binary's mtime (nanosecond epoch), which is a
// build-time proxy rather than a content-addressed version — accurate for the
// normal rebuild path but may invert for binaries rebuilt from an older ref.
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

// detectCopyMode inspects the first existing per-skill child under target to
// decide whether the prior install used copy mode (regular dir) or symlink
// mode. Returns false (symlink mode) when no prior child exists.
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
