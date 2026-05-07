package errfmt

import (
	"fmt"
	"strings"
)

// IllegalTransition is the JSON-serialisable error for an attempted edge that
// has no row in the type's transition table.
type IllegalTransition struct {
	Code      string   `json:"code"`
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	LegalNext []string `json:"legal_next"`
}

// NewIllegalTransition constructs the error with code preset.
func NewIllegalTransition(typ, id, from, to string, next []string) IllegalTransition {
	return IllegalTransition{Code: "illegal_transition", Type: typ, ID: id, From: from, To: to, LegalNext: next}
}

func (e IllegalTransition) Error() string {
	return fmt.Sprintf("illegal transition: %s.%s %s → %s (legal next: %s)",
		e.Type, e.ID, e.From, e.To, strings.Join(e.LegalNext, ", "))
}

// TransitionFlagRequired signals a missing CLI flag for an edge that declares Requires.
type TransitionFlagRequired struct {
	Code string `json:"code"`
	Type string `json:"type"`
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Flag string `json:"flag"`
}

func NewTransitionFlagRequired(typ, id, from, to, flag string) TransitionFlagRequired {
	return TransitionFlagRequired{Code: "transition_flag_required", Type: typ, ID: id, From: from, To: to, Flag: flag}
}

func (e TransitionFlagRequired) Error() string {
	return fmt.Sprintf("missing --%s for transition %s.%s %s → %s", e.Flag, e.Type, e.ID, e.From, e.To)
}

// IndexStale signals that the vault has been edited externally and the
// vault.db needs `anvil reindex`.
type IndexStale struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}

func NewIndexStale() IndexStale {
	return IndexStale{Code: "index_stale", Hint: "anvil reindex"}
}

func (e IndexStale) Error() string {
	return "vault index stale; run `anvil reindex`"
}

// UnsupportedForType signals a per-type gate (e.g. --ready is issue-only today).
type UnsupportedForType struct {
	Code      string   `json:"code"`
	Type      string   `json:"type"`
	Supported []string `json:"supported"`
}

func NewUnsupportedForType(typ string, supported []string) UnsupportedForType {
	return UnsupportedForType{Code: "unsupported_for_type", Type: typ, Supported: supported}
}

func (e UnsupportedForType) Error() string {
	return fmt.Sprintf("flag not supported for type %q (supported: %s)", e.Type, strings.Join(e.Supported, ", "))
}
