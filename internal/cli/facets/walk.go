package facets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// Facets is the closed set of facets enforced by the CLI gate. pattern/ is
// optional everywhere; the gate fires on novelty for any of the three.
var Facets = []string{"domain", "activity", "pattern"}

// CollectValues walks vaultRoot and returns a facet-keyed map of value sets.
// Every key in Facets is present even when its set is empty.
func CollectValues(vaultRoot string) (map[string]map[string]struct{}, error) {
	out := make(map[string]map[string]struct{}, len(Facets))
	for _, f := range Facets {
		out[f] = map[string]struct{}{}
	}

	seenDirs := map[string]struct{}{}
	for _, t := range core.AllTypes {
		dir := filepath.Join(vaultRoot, t.Dir())
		if _, dup := seenDirs[dir]; dup {
			continue
		}
		seenDirs[dir] = struct{}{}
		if err := walkDir(dir, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func walkDir(dir string, out map[string]map[string]struct{}) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
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
}
