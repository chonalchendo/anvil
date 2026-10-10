package cli

import "github.com/chonalchendo/anvil/internal/core"

// bumpOutcome adds one to an outcome counter. An absent counter reads as 0, so
// nothing is written until the first event.
func bumpOutcome(a *core.Artifact, key string) {
	n, _ := a.FrontMatter[key].(int)
	a.FrontMatter[key] = n + 1
}
