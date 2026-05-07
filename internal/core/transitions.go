package core

import "errors"

// ErrIllegalTransition signals no edge from current to target in the type's table.
var ErrIllegalTransition = errors.New("illegal transition")

// Transition is one edge in a per-type state machine.
type Transition struct {
	From, To string
	Requires []string // mandatory CLI flag names
	Reverse  bool     // requires --reason; backward audit edge
}

var transitions = map[Type][]Transition{
	TypeIssue: {
		{From: "open", To: "in-progress", Requires: []string{"owner"}},
		{From: "in-progress", To: "resolved"},
		{From: "in-progress", To: "open"},
		{From: "open", To: "abandoned"},
		{From: "in-progress", To: "abandoned"},
		{From: "resolved", To: "open", Reverse: true},
		{From: "abandoned", To: "open", Reverse: true},
	},
	TypePlan: {
		{From: "draft", To: "locked"},
		{From: "locked", To: "in-progress"},
		{From: "in-progress", To: "done"},
		{From: "in-progress", To: "abandoned"},
		{From: "locked", To: "abandoned"},
		{From: "done", To: "in-progress", Reverse: true},
	},
	TypeMilestone: {
		{From: "planned", To: "in-progress"},
		{From: "in-progress", To: "done"},
		{From: "in-progress", To: "abandoned"},
		{From: "planned", To: "abandoned"},
		{From: "done", To: "in-progress", Reverse: true},
	},
	TypeDecision: {
		{From: "proposed", To: "accepted"},
		{From: "proposed", To: "rejected"},
		{From: "accepted", To: "deprecated"},
		{From: "accepted", To: "superseded"},
	},
	TypeInbox: {
		{From: "raw", To: "promoted"},
		{From: "raw", To: "dropped"},
	},
	TypeThread: {
		{From: "open", To: "paused"},
		{From: "paused", To: "open"},
		{From: "open", To: "closed"},
		{From: "open", To: "abandoned"},
	},
	TypeLearning: {
		{From: "draft", To: "verified"},
		{From: "verified", To: "stale"},
		{From: "stale", To: "verified"},
		{From: "verified", To: "retracted"},
	},
	TypeSweep: {
		{From: "planned", To: "in-progress"},
		{From: "in-progress", To: "merged"},
		{From: "in-progress", To: "abandoned"},
		{From: "planned", To: "abandoned"},
	},
}

// LookupTransition returns the matching edge or ErrIllegalTransition.
func LookupTransition(t Type, from, to string) (Transition, error) {
	for _, tr := range transitions[t] {
		if tr.From == from && tr.To == to {
			return tr, nil
		}
	}
	return Transition{}, ErrIllegalTransition
}

// LegalNext lists every `to` reachable from `from` for type t (unordered).
func LegalNext(t Type, from string) []string {
	var out []string
	for _, tr := range transitions[t] {
		if tr.From == from {
			out = append(out, tr.To)
		}
	}
	return out
}
