package core

// RequiredProductDesignSections is the section order writing-product-design
// Phase 6 prescribes. Validate does not enforce it; it feeds scaffold and
// --show-template only.
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

// RequiredSystemDesignSections is the section order writing-system-design
// "Required sections" prescribes. Validate does not enforce it.
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
