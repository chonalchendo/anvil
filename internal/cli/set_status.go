package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// guardStatusSet keeps set from skipping a type's transition table: a tabled
// type's status moves through `anvil transition`, or through the audited
// --force --reason escape, which appends an audit line to the body.
func guardStatusSet(cmd *cobra.Command, a *core.Artifact, t core.Type, id string, from any, to string, force bool, reason string) error {
	if !core.HasTransitions(t) {
		return nil
	}
	fromS, _ := from.(string)
	if !force {
		hint := fmt.Sprintf("anvil transition %s %s %s", t, id, to)
		if _, err := core.LookupTransition(t, fromS, to); err != nil {
			hint = fmt.Sprintf("anvil set %s %s status %s --force --reason \"<why>\"", t, id, to)
		}
		return printAndReturn(cmd, errfmt.NewStructured("status_via_set").
			Set("type", string(t)).
			Set("id", id).
			Set("from", fromS).
			Set("to", to).
			Set("legal_next", core.LegalNext(t, fromS)).
			Set("fix_hint", hint))
	}
	if reason == "" {
		return printAndReturn(cmd, errfmt.NewStructured("status_force_needs_reason").
			Set("type", string(t)).
			Set("id", id).
			Set("fix_hint", fmt.Sprintf("anvil set %s %s status %s --force --reason \"<why>\"", t, id, to)))
	}
	if !strings.HasSuffix(a.Body, "\n") {
		a.Body += "\n"
	}
	a.Body += fmt.Sprintf("\n> status %s → %s --force %s: %s\n", fromS, to, time.Now().UTC().Format("2006-01-02"), reason)
	return nil
}
