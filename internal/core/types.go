package core

import (
	"fmt"
	"path/filepath"
)

// Type names a vault artifact type. The set is closed in v0.1.
type Type string

// Canonical artifact-type identifiers used across the CLI and schemas.
const (
	TypeInbox         Type = "inbox"
	TypeIssue         Type = "issue"
	TypePlan          Type = "plan"
	TypeMilestone     Type = "milestone"
	TypeDecision      Type = "decision"
	TypeLearning      Type = "learning"
	TypeThread        Type = "thread"
	TypeSweep         Type = "sweep"
	TypeSession       Type = "session"
	TypeProductDesign Type = "product-design"
	TypeSystemDesign  Type = "system-design"
)

// AllTypes lists every Type accepted by the v0.1 CLI.
var AllTypes = []Type{TypeInbox, TypeIssue, TypePlan, TypeMilestone, TypeDecision, TypeLearning, TypeThread, TypeSweep, TypeSession, TypeProductDesign, TypeSystemDesign}

// Dir returns the vault subdirectory that holds artifacts of type t.
// Panics on an unknown Type — callers must validate via ParseType first.
func (t Type) Dir() string {
	switch t {
	case TypeInbox:
		return "00-inbox"
	case TypeIssue:
		return "70-issues"
	case TypePlan:
		return "80-plans"
	case TypeMilestone:
		return "85-milestones"
	case TypeDecision:
		return "30-decisions"
	case TypeLearning:
		return "20-learnings"
	case TypeThread:
		return "60-threads"
	case TypeSweep:
		return "50-sweeps"
	case TypeSession:
		return "10-sessions"
	case TypeProductDesign, TypeSystemDesign:
		return "05-projects"
	}
	panic(fmt.Sprintf("unknown type %q", t))
}

// AllocatesID reports whether create should call NextID for this type.
// Singletons (product-design, system-design) write to a fixed per-project
// filename and return false; every other type returns true.
func (t Type) AllocatesID() bool {
	switch t {
	case TypeProductDesign, TypeSystemDesign:
		return false
	}
	return true
}

// SupportsProject reports whether the type's schema accepts a `project:`
// frontmatter field. Types that return false (decision, inbox, learning,
// session, sweep, thread) are deliberately cross-project — scope them via
// tags (`domain/X`) or topic-prefix conventions.
func (t Type) SupportsProject() bool {
	switch t {
	case TypeIssue, TypePlan, TypeMilestone, TypeProductDesign, TypeSystemDesign:
		return true
	}
	return false
}

// TypesSupportingProject returns the list of type names whose schema accepts
// a `project:` field, in AllTypes order. Used for help text and error
// suggestions.
func TypesSupportingProject() []string {
	var out []string
	for _, t := range AllTypes {
		if t.SupportsProject() {
			out = append(out, string(t))
		}
	}
	return out
}

// Path returns the absolute artifact path under vaultRoot. Singletons
// (product-design, system-design) ignore id and use a fixed per-project
// filename; other types compose <Dir>/<id>.md.
func (t Type) Path(vaultRoot, project, id string) string {
	if !t.AllocatesID() {
		return filepath.Join(vaultRoot, t.Dir(), project, string(t)+".md")
	}
	return filepath.Join(vaultRoot, t.Dir(), id+".md")
}

// ParseType returns the Type matching s, or an error if s is not a known type.
func ParseType(s string) (Type, error) {
	for _, t := range AllTypes {
		if string(t) == s {
			return t, nil
		}
	}
	return "", fmt.Errorf("unknown type %q", s)
}
