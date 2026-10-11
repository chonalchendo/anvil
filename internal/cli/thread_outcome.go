package cli

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

var outcomeLinkRe = regexp.MustCompile(`\[\[((?:decision|learning)\.[^\]]+)\]\]`)

// threadOutcomeLinked reports whether a closing thread routes its outcome to a
// decision or learning that exists. Both the body (hand-authored resolution
// prose) and `related` — the only link slot thread.schema.json allows, and
// where `anvil link` writes — count, as does a decision whose `thread` slot
// names this thread (`create decision --thread`), so a properly-linked thread
// never draws the warning. Fenced blocks are stripped (an illustrative
// wikilink in a code sample is not an outcome) and a dangling target does not
// count: a typo'd id must not silence the gate.
func threadOutcomeLinked(v *core.Vault, a *core.Artifact) bool {
	if outcomeLinkResolves(v, core.StripFencedBlocks(a.Body)) {
		return true
	}
	vals, _ := a.FrontMatter["related"].([]any)
	for _, e := range vals {
		if s, ok := e.(string); ok && outcomeLinkResolves(v, s) {
			return true
		}
	}
	// `create decision --thread` writes the decision's `thread` slot at mint
	// time, so no `link` edge exists on the thread itself.
	want := "[[thread." + strings.TrimSuffix(filepath.Base(a.Path), ".md") + "]]"
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

// outcomeLinkResolves reports whether s carries a decision or learning wikilink
// whose target names a file present in v.
func outcomeLinkResolves(v *core.Vault, s string) bool {
	for _, m := range outcomeLinkRe.FindAllStringSubmatch(s, -1) {
		if core.WikilinkTargetExists(v, m[1]) {
			return true
		}
	}
	return false
}
