package core

// RequiredProductDesignSections owns the product-design section order; writing-product-design points here via --show-template.
// Validate does not enforce it; only --show-template
// prints it. The no-body create keeps an empty design body.
var RequiredProductDesignSections = []string{
	"## TL;DR",
	"## What we're building",
	"## Who it's for",
	"## Why it matters",
	"## Approach",
	"## Goals and how we measure them",
	"## Constraints & appetite",
	"## What's deliberately out of scope",
	"## Risks, rabbit holes, open questions",
	"## Milestones",
}

// RequiredSystemDesignSections owns the system-design section order; writing-system-design points here via --show-template. Validate does not enforce it.
var RequiredSystemDesignSections = []string{
	"## TL;DR",
	"## Context and scope",
	"## Non-goals",
	"## Constraints and quality goals",
	"## Components",
	"## Runtime flow",
	"## System invariants",
	"## Decisions",
	"## Open questions",
}

// RequiredDataModelSections owns the data-model section order; writing-data-model points here via --show-template. Validate enforces it.
var RequiredDataModelSections = []string{
	"## Domains",
	"## Conceptual",
	"## Logical",
	"## Physical",
	"## Open questions",
}

// ValidateDataModel checks the required headings appear in order.
func ValidateDataModel(a *Artifact) []error {
	return scanOrderedHeadings(a.Body, "data-model", RequiredDataModelSections)
}
