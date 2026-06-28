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
	TypeContract      Type = "contract"
	TypeConvention    Type = "convention"
)

// AllTypes lists every Type accepted by the v0.1 CLI.
var AllTypes = []Type{TypeInbox, TypeIssue, TypePlan, TypeMilestone, TypeDecision, TypeLearning, TypeThread, TypeSweep, TypeSession, TypeProductDesign, TypeSystemDesign, TypeContract, TypeConvention}

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
	case TypeProductDesign:
		return "05-product-designs"
	case TypeSystemDesign:
		return "06-system-designs"
	case TypeContract:
		return "75-contracts"
	case TypeConvention:
		return "35-conventions"
	}
	panic(fmt.Sprintf("unknown type %q", t))
}

// SupportsProject reports whether the type's schema accepts a `project:`
// frontmatter field. Types that return false (inbox, session, sweep, thread)
// are deliberately cross-project — inbox predates project assignment, sessions
// cross repos, and sweep/thread are spans by construction.
func (t Type) SupportsProject() bool {
	switch t {
	case TypeIssue, TypePlan, TypeMilestone, TypeProductDesign, TypeSystemDesign, TypeLearning, TypeDecision, TypeContract:
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

// Path returns the absolute artifact path under vaultRoot: <Dir>/<id>.md.
func (t Type) Path(vaultRoot, id string) string {
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
