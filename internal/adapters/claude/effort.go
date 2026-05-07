// Package claude implements build.AgentAdapter for the Claude Code CLI.
package claude

// translateEffort maps anvil's effort tier (low|medium|high|xhigh) onto
// Claude Code's thinking-budget tier. The mapping is total — no clamp ever
// occurs — so the slog.Warn dedup the spec §1 mandates "on any clamp" lives
// in the Codex adapter, not here. Validator guarantees `effort` is one of
// the four values; an unknown value falls through to "low" (the orchestrator
// default's translation) so a stale schema doesn't crash a build.
func translateEffort(effort string) string {
	switch effort {
	case "low":
		return "minimal"
	case "medium":
		return "low"
	case "high":
		return "medium"
	case "xhigh":
		return "high"
	default:
		return "low"
	}
}
