package core

import "fmt"

// Type names a vault artifact type. The set is closed in v0.1.
type Type string

const (
	TypeInbox     Type = "inbox"
	TypeIssue     Type = "issue"
	TypePlan      Type = "plan"
	TypeMilestone Type = "milestone"
	TypeDecision  Type = "decision"
)

// AllTypes lists every Type accepted by the v0.1 CLI.
var AllTypes = []Type{TypeInbox, TypeIssue, TypePlan, TypeMilestone, TypeDecision}

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
	}
	panic(fmt.Sprintf("unknown type %q", t))
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
