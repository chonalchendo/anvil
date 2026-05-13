package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Stub is a stray `.md` file at the vault root whose basename starts with
// `<type>.` for one of the known artifact types — typically created by an
// Obsidian wikilink click that resolves to a missing artifact and stamps an
// empty file next to the vault root. These pollute the vault but are silently
// skipped by reindex because they have no frontmatter.
type Stub struct {
	Path string
	Size int64
}

// FindStubs scans the vault root (one level only — not recursive) for files
// matching `<type>.*.md` for any known Type. Files inside canonical
// `<NN>-<type>/` directories are out of scope by construction since we only
// read directory-root entries.
func FindStubs(vaultRoot string) ([]Stub, error) {
	entries, err := os.ReadDir(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("reading vault root: %w", err)
	}
	var out []Stub
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if !matchesTypePrefix(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", name, err)
		}
		out = append(out, Stub{
			Path: filepath.Join(vaultRoot, name),
			Size: info.Size(),
		})
	}
	return out, nil
}

// matchesTypePrefix reports whether name begins with `<known-type>.` for any
// Type in AllTypes — the Obsidian wikilink-resolution pattern.
func matchesTypePrefix(name string) bool {
	for _, t := range AllTypes {
		if strings.HasPrefix(name, string(t)+".") {
			return true
		}
	}
	return false
}
