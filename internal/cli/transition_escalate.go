package cli

import (
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
	case "escalated":
		a.FrontMatter["escalation_reason"] = reason
	default:
		delete(a.FrontMatter, "escalation_reason")
	}
}
