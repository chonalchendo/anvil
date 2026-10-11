package cli

import (
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// decisionNamesThread reports whether some decision's `thread` slot points at
// the thread stored at threadPath. `create decision --thread` writes that slot
// at mint time, so no separate `link` edge exists on the thread itself.
func decisionNamesThread(v *core.Vault, threadPath string) bool {
	want := "[[thread." + strings.TrimSuffix(filepath.Base(threadPath), ".md") + "]]"
	paths, _ := filepath.Glob(filepath.Join(v.Root, core.TypeDecision.Dir(), "*.md"))
	for _, p := range paths {
		d, err := core.LoadArtifact(p)
		if err != nil {
			continue
		}
		if s, _ := d.FrontMatter["thread"].(string); s == want {
			return true
		}
	}
	return false
}
