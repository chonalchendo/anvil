package cli

import (
	"os"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// stampIssueGate records the vault evidence fleet status reads: when an issue
// was claimed, and why it escalated. Leaving escalated clears the reason so a
// re-queued issue does not carry a stale one.
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

// bumpOutcome adds one to an outcome counter. An absent counter reads as 0, so
// nothing is written until the first event.
func bumpOutcome(a *core.Artifact, key string) {
	n, _ := a.FrontMatter[key].(int)
	a.FrontMatter[key] = n + 1
}
