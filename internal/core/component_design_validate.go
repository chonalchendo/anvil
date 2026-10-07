package core

// RequiredComponentDesignSections is the interface-and-ownership core every
// component design carries; internal design (Code design, Decisions, Open
// questions) is optional and gated in writing-component-design, not here.
var RequiredComponentDesignSections = []string{
	"## Does",
	"## Does not",
	"## Interfaces",
	"## Invariants",
	"## Verification",
	"### Direct",
	"### Indirect",
}

// ValidateComponentDesign checks the core headings appear in order.
func ValidateComponentDesign(a *Artifact) []error {
	return scanOrderedHeadings(a.Body, "component-design", RequiredComponentDesignSections)
}
