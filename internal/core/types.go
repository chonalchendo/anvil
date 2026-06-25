package core

import (
	"fmt"
	"path/filepath"
	"strings"
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
)

// AllTypes lists every Type accepted by the v0.1 CLI.
var AllTypes = []Type{TypeInbox, TypeIssue, TypePlan, TypeMilestone, TypeDecision, TypeLearning, TypeThread, TypeSweep, TypeSession, TypeProductDesign, TypeSystemDesign, TypeContract}

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
	case TypeContract:
		return "75-contracts"
	}
	panic(fmt.Sprintf("unknown type %q", t))
}

// AllocatesID reports whether create should call NextID for this type.
// product-design is a per-project singleton and returns false.
// system-design allocates per-shard IDs (<project>.<shard>) and returns true.
func (t Type) AllocatesID() bool {
	return t != TypeProductDesign
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

// Path returns the absolute artifact path under vaultRoot.
// product-design (singleton) ignores id and writes to 05-projects/<project>/product-design.md.
// system-design with id=<project>.<shard> writes to 05-projects/<project>/system-design.<shard>.md;
// id without a dot (bare project or empty) falls back to the legacy singleton path.
// Other types compose <Dir>/<id>.md.
func (t Type) Path(vaultRoot, project, id string) string {
	if !t.AllocatesID() {
		return filepath.Join(vaultRoot, t.Dir(), project, string(t)+".md")
	}
	if t == TypeSystemDesign {
		dot := strings.Index(id, ".")
		if dot < 0 {
			// Bare project or empty id — legacy singleton path.
			proj := id
			if proj == "" {
				proj = project
			}
			return filepath.Join(vaultRoot, t.Dir(), proj, string(t)+".md")
		}
		proj, shard := id[:dot], id[dot+1:]
		return filepath.Join(vaultRoot, t.Dir(), proj, string(t)+"."+shard+".md")
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
