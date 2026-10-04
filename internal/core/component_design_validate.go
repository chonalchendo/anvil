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
// Run on create, validate and append.
func ValidateComponentDesign(a *Artifact) []error {
	return scanOrderedHeadings(a.Body, "component-design", RequiredComponentDesignSections)
}
