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

// ResolveProject resolves the current project. Precedence:
// $ANVIL_PROJECT (if it names an adopted binding) → adopted binding for
// cwd's git tree → git remote → current-project pointer → error.
func ResolveProject() (*Project, error) {
	if slug := os.Getenv("ANVIL_PROJECT"); slug != "" {
		if p, err := projectFromSlug(slug); err == nil {
			return p, nil
		}
		// Env names an unknown slug: fall through to other resolution paths
		// rather than erroring, matching the kind-default precedence model.
	}
	root, err := gitToplevel()
	if err == nil {
		if p, err := readAdoptedBinding(root); err == nil {
			return p, nil
		}
		if remote, err := gitRemoteOrigin(root); err == nil {
			return &Project{Slug: slugFromRemote(remote), Root: root}, nil
		}
	}
	// Only consult the pointer file when not inside a git tree.
	if err != nil {
		if p, perr := readCurrentProjectPointer(); perr == nil {
			return p, nil
		}
	}
	return nil, ErrNoProject
}

// SwitchProject writes slug to ~/.anvil/current-project after verifying the
// slug has an adopted binding.
func SwitchProject(slug string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("switch: resolving home: %w", err)
	}
	binding := filepath.Join(home, ".anvil", "projects", slug, ".binding")
	if _, err := os.Stat(binding); err != nil {
		return fmt.Errorf("switch: slug %q not adopted: %w", slug, err)
	}
	ptr := filepath.Join(home, ".anvil", "current-project")
	if err := os.WriteFile(ptr, []byte(slug+"\n"), 0o644); err != nil {
		return fmt.Errorf("switch: writing pointer: %w", err)
	}
	return nil
}

// ListProjects returns all adopted projects found under ~/.anvil/projects/.
func ListProjects() ([]Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("list: resolving home: %w", err)
	}
	base := filepath.Join(home, ".anvil", "projects")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list: reading projects dir: %w", err)
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(base, e.Name(), ".binding"))
		if err != nil {
			continue
		}
		out = append(out, Project{Slug: e.Name(), Root: strings.TrimSpace(string(b))})
	}
	return out, nil
}

// readCurrentProjectPointer reads ~/.anvil/current-project and resolves the
// slug to a full Project by looking up its adopted binding.
func readCurrentProjectPointer() (*Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(home, ".anvil", "current-project"))
	if err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(string(b))
	binding := filepath.Join(home, ".anvil", "projects", slug, ".binding")
	rb, err := os.ReadFile(binding)
	if err != nil {
		return nil, fmt.Errorf("current-project %q has no binding: %w", slug, err)
	}
	return &Project{Slug: slug, Root: strings.TrimSpace(string(rb))}, nil
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

// projectFromSlug returns the Project for an adopted slug, or an error if no
// binding exists.
func projectFromSlug(slug string) (*Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(home, ".anvil", "projects", slug, ".binding"))
	if err != nil {
		return nil, err
	}
	return &Project{Slug: slug, Root: strings.TrimSpace(string(b))}, nil
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
