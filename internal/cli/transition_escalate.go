package cli

import (
	"os"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// stampIssueGate records the vault evidence fleet status reads: when an issue
// was claimed, and why it escalated. Leaving escalated clears the reason so a
// re-queued issue does not carry a stale one. Each entry to escalated also bumps
// outcome_escalations: the reason is cleared on exit, so the count is the only
// record that an issue escalated at all.
func stampIssueGate(a *core.Artifact, to, reason string, now time.Time) {
	switch to {
	case "in-progress":
		a.FrontMatter["claimed_at"] = now.Format(time.RFC3339)
		a.FrontMatter["verification_lock"] = core.VerificationLock(a.Body)
		// The claiming session lets a later same-owner claim from a different
		// session be refused. Omitted outside a Claude session (env unset).
		if sid := os.Getenv(envSessionID); sid != "" {
			a.FrontMatter["claim_session"] = sid
		}
	case "escalated":
		a.FrontMatter["escalation_reason"] = reason
		bumpOutcome(a, "outcome_escalations")
	default:
		delete(a.FrontMatter, "escalation_reason")
	}
}
