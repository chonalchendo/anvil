package core

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chonalchendo/anvil/internal/schema"
)

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
		{From: "in-progress", To: "escalated", Requires: []string{"reason"}},
		{From: "escalated", To: "open"},
		{From: "escalated", To: "abandoned"},
		{From: "resolved", To: "open", Reverse: true},
		{From: "abandoned", To: "open", Reverse: true},
	},
	TypeMilestone: {
		{From: "planned", To: "in-progress"},
		{From: "in-progress", To: "done"},
		{From: "in-progress", To: "planned"},
		{From: "in-progress", To: "abandoned"},
		{From: "planned", To: "abandoned"},
		{From: "done", To: "in-progress", Reverse: true},
		{From: "done", To: "planned", Reverse: true},
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
		{From: "raw", To: "triaged"},
		{From: "triaged", To: "promoted"},
		{From: "triaged", To: "dropped"},
	},
	TypeThread: {
		{From: "open", To: "paused"},
		{From: "paused", To: "open"},
		{From: "open", To: "closed"},
		{From: "open", To: "abandoned"},
	},
	TypeLearning: {
		{From: "draft", To: "verified"},
		{From: "draft", To: "stale"},
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
	TypeComponentDesign: {
		{From: "draft", To: "active"},
		{From: "active", To: "deprecated"},
		{From: "deprecated", To: "active", Reverse: true},
	},
	TypeConvention: {
		{From: "draft", To: "active"},
		{From: "active", To: "deprecated"},
		{From: "active", To: "superseded"},
		{From: "deprecated", To: "active", Reverse: true},
	},
	TypeProductDesign: designTable,
	TypeSystemDesign:  designTable,
	TypeSession: {
		{From: "raw", To: "triaged"},
		{From: "triaged", To: "distilled"},
		{From: "distilled", To: "archived"},
		{From: "raw", To: "archived"},
	},
}

// designTable is shared: product-design and system-design have one lifecycle.
var designTable = []Transition{
	{From: "draft", To: "active"},
	{From: "active", To: "superseded"},
	{From: "active", To: "retired"},
	{From: "superseded", To: "active", Reverse: true},
	{From: "retired", To: "active", Reverse: true},
}

// InitialStatus returns the status a new artifact of type t starts in. By
// invariant the first value of the schema's status enum is the initial status
// for all twelve types; the schema owns the enum.
func InitialStatus(t Type) string {
	b, err := schema.EmbeddedFS.ReadFile(string(t) + ".schema.json")
	if err != nil {
		panic(fmt.Sprintf("InitialStatus: reading embedded schema for %s: %v", t, err))
	}
	var raw struct {
		Properties struct {
			Status struct {
				Enum []string `json:"enum"`
			} `json:"status"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		panic(fmt.Sprintf("InitialStatus: parsing embedded schema for %s: %v", t, err))
	}
	return raw.Properties.Status.Enum[0]
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
