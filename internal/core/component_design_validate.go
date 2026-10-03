package core

// RequiredComponentDesignSections is the boundary half every component design
// carries; the design half (Interfaces, Shape, Flow, Invariants, Decisions,
// Risks) is optional and gated in writing-component-design, not here.
var RequiredComponentDesignSections = []string{
	"## Purpose",
	"## Does",
	"## Does not",
	"## Verification",
	"### Direct",
	"### Indirect",
	"## Precedents",
}

// ValidateComponentDesign checks the boundary-half headings appear in order.
// Run at create time only: the vault's pre-rename contracts carry no
// `## Purpose`, so a vault-wide or append-time check would reject them.
func ValidateComponentDesign(a *Artifact) []error {
	return scanOrderedHeadings(a.Body, "component-design", RequiredComponentDesignSections)
}
