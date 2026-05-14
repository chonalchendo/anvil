package errfmt

import "fmt"

// NewIllegalTransition builds the structured error for an attempted edge that
// has no row in the type's transition table. Always includes a `hint` field
// carrying the raw, copy-pasteable `anvil set` command that bypasses the
// state machine — so agents consuming the JSON envelope literally get an
// executable next step. A separate `hint_note` carries the caveat (no audit
// trail) for humans reading the text rendering.
func NewIllegalTransition(typ, id, from, to string, next []string) *Structured {
	hint := fmt.Sprintf("anvil set %s %s status %s", typ, id, to)
	return NewStructured("illegal_transition").
		Set("type", typ).
		Set("id", id).
		Set("from", from).
		Set("to", to).
		Set("legal_next", next).
		Set("hint", hint).
		Set("hint_note", "force-edit: bypasses state machine, no audit trail")
}

// NewTransitionFlagRequired builds the structured error for a missing CLI flag
// on an edge that declares Requires. Includes a corrected, copy-pasteable
// invocation per agent-cli-principles rule 4.
func NewTransitionFlagRequired(typ, id, from, to, flag string) *Structured {
	corrected := fmt.Sprintf("anvil transition %s %s %s --%s <%s>", typ, id, to, flag, flagValuePlaceholder(flag))
	return NewStructured("transition_flag_required").
		Set("type", typ).
		Set("id", id).
		Set("from", from).
		Set("to", to).
		Set("flag", flag).
		Set("required", true).
		Set("corrected", corrected)
}

// NewIndexStale signals that the vault has been edited externally and the
// vault.db needs `anvil reindex`.
func NewIndexStale() *Structured {
	return NewStructured("index_stale").Set("hint", "anvil reindex")
}

// NewUnsupportedForType signals a per-type gate (e.g. --ready is issue-only).
func NewUnsupportedForType(typ string, supported []string) *Structured {
	return NewStructured("unsupported_for_type").
		Set("type", typ).
		Set("supported", supported)
}

// NewUnsupportedFlagForType signals that a CLI flag is inert for the given
// type because the type's schema rejects the corresponding frontmatter field.
// Includes a `suggest` hint pointing to the convention to use instead.
func NewUnsupportedFlagForType(flag, typ string, supported []string, suggest string) *Structured {
	return NewStructured("unsupported_flag_for_type").
		Set("flag", flag).
		Set("type", typ).
		Set("supported", supported).
		Set("suggest", suggest)
}

func flagValuePlaceholder(flag string) string {
	switch flag {
	case "owner":
		return "name"
	case "reason":
		return "audit reason"
	default:
		return "value"
	}
}
