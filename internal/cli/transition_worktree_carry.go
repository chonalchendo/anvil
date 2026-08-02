package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

// carryFileName names untracked paths a project wants copied into each cut
// worktree — a live Iron Law check needs credentials git does not track
// (anvil.0178). Format: docs/worktree-workflow.md.
const carryFileName = ".anvil-worktree-carry"

// checkCarryDeclarations reads and validates repoDir's carry list before the
// worktree exists: a refusal here leaves nothing to clean up, where a refusal
// after the cut would orphan the worktree and branch. Entries must be
// repo-relative and stay inside the repo — an absolute path or a `..` escape
// would silently read outside the repo and write outside the worktree.
// Returns the declared paths for copyCarryFiles once the cut lands.
func checkCarryDeclarations(repoDir string) ([]string, error) {
	paths, err := readCarryList(repoDir)
	if err != nil {
		return nil, errfmt.NewStructured("cut_worktree_carry_failed").
			Set("error", err.Error())
	}
	for _, rel := range paths {
		if filepath.IsAbs(rel) || strings.HasPrefix(filepath.Clean(rel), "..") {
			return nil, errfmt.NewStructured("cut_worktree_carry_invalid").
				Set("path", rel).
				Set("fix_hint", fmt.Sprintf("%s entries must be repo-relative paths inside the repo; %q is absolute or escapes via ..", carryFileName, rel))
		}
		if _, serr := os.Stat(filepath.Join(repoDir, rel)); serr != nil {
			return nil, errfmt.NewStructured("cut_worktree_carry_missing").
				Set("path", rel).
				Set("repo", repoDir).
				Set("fix_hint", fmt.Sprintf("%s declares %q but it does not exist in %s", carryFileName, rel, repoDir))
		}
	}
	return paths, nil
}

// copyCarryFiles copies each already-validated path from repoDir into
// worktreePath, preserving its relative location.
func copyCarryFiles(repoDir, worktreePath string, paths []string) error {
	for _, rel := range paths {
		src := filepath.Join(repoDir, rel)
		info, serr := os.Stat(src)
		if serr != nil {
			return errfmt.NewStructured("cut_worktree_carry_failed").
				Set("path", rel).
				Set("error", serr.Error())
		}
		if err := copyCarryPath(src, filepath.Join(worktreePath, rel), info); err != nil {
			return errfmt.NewStructured("cut_worktree_carry_failed").
				Set("path", rel).
				Set("error", err.Error())
		}
	}
	return nil
}

// readCarryList reads repoDir's carryFileName; an absent file means nothing
// declared (nil, not an error).
func readCarryList(repoDir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(repoDir, carryFileName)) //nolint:gosec // G304: fixed name joined onto the resolved repo root, not user input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// copyCarryPath copies src (file or directory) to dst, creating parent dirs
// as needed and mirroring each source file's mode so carried executables
// stay executable.
func copyCarryPath(src, dst string, info os.FileInfo) error {
	if !info.IsDir() {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec // G301: directories must be traversable
			return err
		}
		return copyFileContents(src, dst, info.Mode().Perm())
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755) //nolint:gosec // G301: directories must be traversable
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		return copyFileContents(p, target, fi.Mode().Perm())
	})
}

func copyFileContents(src, dst string, mode os.FileMode) error {
	b, err := os.ReadFile(src) //nolint:gosec // G304: entries are validated repo-relative by checkCarryDeclarations
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, mode) //nolint:gosec // G306: mirrors the source file's mode; carried executables must stay executable
}
