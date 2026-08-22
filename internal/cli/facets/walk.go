package facets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
)

// validFacets is the closed set of facets enforced by the CLI gate. pattern/
// is optional everywhere; the gate fires on novelty for any of the three.
var validFacets = []string{"domain", "activity", "pattern"}

// Has reports whether name is a recognised facet.
func Has(name string) bool { return slices.Contains(validFacets, name) }

// Names returns a fresh copy of the facet list — safe for callers to retain
// or mutate without affecting validation behaviour.
func Names() []string { return slices.Clone(validFacets) }

// CollectValues walks vaultRoot and returns a facet-keyed map of value sets:
// the union of values already in use on artifacts and values registered in
// the glossary (`anvil tags add`). Every recognised facet key is present even
// when its set is empty. Reading the glossary as a source (not just usage)
// is what lets a seeded-but-unused value pass the novelty gate.
//
// Artifacts whose YAML frontmatter cannot be parsed are skipped rather than
// aborting the walk; their paths are returned in the second return value so
// callers can surface a warning. Real OS errors (permissions, missing parent
// directory, etc.) still propagate as a non-nil error.
func CollectValues(vaultRoot string) (map[string]map[string]struct{}, []string, error) {
	out := make(map[string]map[string]struct{}, len(validFacets))
	for _, f := range validFacets {
		out[f] = map[string]struct{}{}
	}

	var skipped []string

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
			a, aErr := core.LoadArtifact(path)
			if aErr != nil {
				if errors.Is(aErr, core.ErrFrontmatterParse) {
					// Tolerate corrupt frontmatter: record the path and continue
					// the walk so one bad artifact doesn't block creates everywhere.
					skipped = append(skipped, path)
					return nil
				}
				return aErr
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
			return nil, nil, err
		}
	}

	g, gErr := glossary.Load(glossary.Path(vaultRoot))
	if gErr != nil {
		// Unlike the artifact walk above, an I/O error here propagates rather
		// than being tolerated: glossary.Load already treats missing/empty as
		// New(), so an error here means the path is unreadable or directory-
		// shaped — a real environment fault, not a malformed-content case.
		return nil, nil, fmt.Errorf("loading glossary: %w", gErr)
	}
	for _, tag := range g.Tags() {
		if !core.ValidTagShape(tag) {
			// anvil tags add never checks value shape, so the glossary can
			// carry an entry (e.g. pattern/Bad_Value) that fails the vault's
			// own tag-shape rule. Admitting it here would let the gate pass
			// a tag that anvil validate immediately rejects, so it's
			// filtered out of the union rather than trusted as a real value.
			continue
		}
		facet, value, hasSlash := strings.Cut(tag, "/")
		if !hasSlash || value == "" {
			continue
		}
		bucket, ok := out[facet]
		if !ok {
			continue
		}
		bucket[value] = struct{}{}
	}

	return out, skipped, nil
}
