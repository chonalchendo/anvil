// Package build orchestrates the wave-walking loop for `anvil build`. It owns
// the AgentAdapter contract; per-CLI implementations live in internal/adapters.
package build

import (
	"context"
	"errors"
	"time"
)

// AgentAdapter is implemented per agent CLI (Claude Code, Codex). The build
// orchestrator routes by core.Task.Model prefix and calls Run for each task.
type AgentAdapter interface {
	Name() string
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}

// RunRequest is the build-assembled spawn input. Adapters wrap Instruction in
// their CLI envelope and load Skills via the CLI's skill mechanism; Context
// files are surfaced as plain context.
type RunRequest struct {
	Model       string
	Effort      string
	Instruction string
	Skills      []string
	Context     []string
	Files       []string
	Cwd         string
	Timeout     time.Duration
}

// RunResult is what the adapter reports back. On the error path the adapter
// populates whatever fields it parsed before failure (always Duration);
// build does not synthesise zeros for unset fields.
type RunResult struct {
	ExitCode  int
	Duration  time.Duration
	AgentTime time.Duration
	Tokens    TokenUsage
	CostUSD   float64
	// Diagnostic carries the agent's last result-event text (or captured
	// stderr if no result event arrived). Adapters populate it on failure
	// paths so build's --json record and stderr output can surface why a
	// task failed without forcing the user into the agent's session log.
	Diagnostic string
}

// TokenUsage is per-task token accounting reported by an adapter alongside RunResult.
type TokenUsage struct {
	Input, Output, CacheRead, CacheWrite int64
}

// ErrQuotaExhausted is returned by Run when the agent CLI surfaces a usage-cap
// / rate-limit signal where resumption-after-reset is the right response.
// Distinct from a non-zero RunResult.ExitCode (which means the task itself
// failed).
var ErrQuotaExhausted = errors.New("agent quota exhausted")

// Router maps a model-name prefix to its adapter. Build's selectAdapter walks
// keys longest-first so "claude-" wins over "" if both are present.
type Router map[string]AgentAdapter
