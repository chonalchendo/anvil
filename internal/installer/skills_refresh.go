package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// RefreshSkillsIfStale rewrites the installed bundle when the hash recorded
// in materialiseDir diverges from srcFS. No-op when materialiseDir is absent.
// The install mode (symlink vs copy) is recovered by inspecting any existing
// per-skill child under target.
//
// Refresh is monotonic: a released user upgrades the binary, so embedded skills
// are always newer-or-equal than what is installed, and refreshing whenever the
// hashes differ is correct. The one path that could regress — building from an
// older git ref and letting it downgrade a newer installed bundle — is a
// dogfood artifact, not a user scenario; recover from it with an explicit
// `anvil install skills --force`.
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
