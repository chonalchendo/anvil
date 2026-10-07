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
		// The claiming session lets a later same-owner claim from a different
		// session be refused. Omitted outside a Claude session (env unset).
		if sid := os.Getenv(envSessionID); sid != "" {
			a.FrontMatter["claim_session"] = sid
		}
	case "escalated":
		a.FrontMatter["escalation_reason"] = reason
	default:
		delete(a.FrontMatter, "escalation_reason")
	}
}
