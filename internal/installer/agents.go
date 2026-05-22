package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InstallAgents copies each top-level *.md agent in srcFS to target/<name>.md.
// target is the user-agents dir (typically ~/.claude/agents), shared with
// non-anvil agent files — so it writes per-file rather than mirroring the tree
// the way InstallSkills does. Agents are single files with no symlink/refresh
// machinery; idempotent re-copy is the whole story.
//
// An existing file whose content already matches the embedded copy is a no-op;
// one that diverges is refused unless force is true, mirroring InstallSkills's
// promise not to clobber user-owned content silently.
func InstallAgents(srcFS fs.FS, target string, force bool) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false, fmt.Errorf("mkdir target %s: %w", target, err)
	}
	changed := false
	for _, name := range names {
		want, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst)
		switch {
		case err == nil && bytes.Equal(got, want):
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --force` to redeploy", dst)
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, want, 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// RemoveAgents deletes target/<name>.md for each embedded agent whose on-disk
// content still matches the embedded copy (anvil-owned). Divergent or foreign
// files are left untouched, mirroring RemoveSkills.
func RemoveAgents(srcFS fs.FS, target string) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	changed := false
	for _, name := range names {
		want, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if !bytes.Equal(got, want) {
			continue
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

func listAgentFiles(srcFS fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read agents FS: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}
