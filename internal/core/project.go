package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Project identifies the source repository whose vault artifacts we operate on.
type Project struct {
	Slug string
	Root string // absolute path to the project's git working tree
}

// ErrNoProject signals that the working directory is not a git repo and has no
// adopted binding.
var ErrNoProject = errors.New("no project: not a git repo and no anvil binding")

// ResolveProject implements the three-step fallback per system-design.md:
// adopted binding → git remote URL → error.
func ResolveProject() (*Project, error) {
	root, err := gitToplevel()
	if err == nil {
		if p, err := readAdoptedBinding(root); err == nil {
			return p, nil
		}
		if remote, err := gitRemoteOrigin(root); err == nil {
			return &Project{Slug: slugFromRemote(remote), Root: root}, nil
		}
	}
	return nil, ErrNoProject
}

// AdoptProject records an explicit binding for the current git tree.
func AdoptProject(slug string) error {
	root, err := gitToplevel()
	if err != nil {
		return fmt.Errorf("adopt: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("adopt: resolving home: %w", err)
	}
	dir := filepath.Join(home, ".anvil", "projects", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("adopt: mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".binding"), []byte(root+"\n"), 0o644); err != nil {
		return fmt.Errorf("adopt: write binding: %w", err)
	}
	return nil
}

func gitToplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRemoteOrigin(dir string) (string, error) {
	c := exec.Command("git", "remote", "get-url", "origin")
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var slugRe = regexp.MustCompile(`[^/]+?(?:\.git)?$`)

func slugFromRemote(remote string) string {
	m := slugRe.FindString(remote)
	return strings.TrimSuffix(m, ".git")
}

func readAdoptedBinding(root string) (*Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".anvil", "projects")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(base, e.Name(), ".binding"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(b)) == root {
			return &Project{Slug: e.Name(), Root: root}, nil
		}
	}
	return nil, fmt.Errorf("no adopted binding for %s", root)
}
