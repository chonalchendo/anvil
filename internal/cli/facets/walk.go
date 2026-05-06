package facets

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// validFacets is the closed set of facets enforced by the CLI gate. pattern/
// is optional everywhere; the gate fires on novelty for any of the three.
var validFacets = []string{"domain", "activity", "pattern"}

// Has reports whether name is a recognised facet.
func Has(name string) bool { return slices.Contains(validFacets, name) }

// Names returns a fresh copy of the facet list — safe for callers to retain
// or mutate without affecting validation behaviour.
func Names() []string { return slices.Clone(validFacets) }

// CollectValues walks vaultRoot and returns a facet-keyed map of value sets.
// Every recognised facet key is present even when its set is empty.
func CollectValues(vaultRoot string) (map[string]map[string]struct{}, error) {
	out := make(map[string]map[string]struct{}, len(validFacets))
	for _, f := range validFacets {
		out[f] = map[string]struct{}{}
	}

	seenDirs := map[string]struct{}{}
	for _, t := range core.AllTypes {
		dir := filepath.Join(vaultRoot, t.Dir())
		if _, dup := seenDirs[dir]; dup {
			continue
		}
		seenDirs[dir] = struct{}{}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			a, err := core.LoadArtifact(path)
			if err != nil {
				return fmt.Errorf("loading %s: %w", path, err)
			}
			raw, ok := a.FrontMatter["tags"].([]any)
			if !ok {
				return nil
			}
			for _, item := range raw {
				s, ok := item.(string)
				if !ok || s == "" {
					continue
				}
				facet, value, hasSlash := strings.Cut(s, "/")
				if !hasSlash || value == "" {
					continue
				}
				bucket, ok := out[facet]
				if !ok {
					continue
				}
				bucket[value] = struct{}{}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
