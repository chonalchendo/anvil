package core

import (
	"os"
	"path/filepath"
	"strings"
)

// maxNearestIDs caps how many suggestions a rejection lists.
const maxNearestIDs = 3

// NearestArtifactTargets returns up to maxNearestIDs existing artifacts of the
// target's type whose id extends the target's id at a segment boundary (a
// short-form link), as `<type>.<id>` wikilink targets in sorted order, plus
// the count of further matches left out. Nil when the target names no known
// type or nothing extends it.
func NearestArtifactTargets(v *Vault, target string) (near []string, more int) {
	dot := strings.IndexByte(target, '.')
	if dot < 0 {
		return nil, 0
	}
	t, err := ParseType(target[:dot])
	if err != nil {
		return nil, 0
	}
	bare := BareID(t, target)
	if bare == "" {
		return nil, 0
	}
	entries, err := os.ReadDir(filepath.Join(v.Root, t.Dir()))
	if err != nil {
		return nil, 0
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok {
			continue
		}
		id := BareID(t, name)
		// Segment-bounded: "serving.001" must not suggest "serving.0010-x".
		if len(id) <= len(bare) || !strings.HasPrefix(id, bare) || (id[len(bare)] != '.' && id[len(bare)] != '-') {
			continue
		}
		if len(near) == maxNearestIDs {
			more++
			continue
		}
		near = append(near, WikilinkTarget(t, id))
	}
	return near, more
}
