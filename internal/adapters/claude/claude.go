package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// Adapter spawns the Claude Code CLI per task. One Adapter is shared across
// every task in a build — the per-spawn state (CLAUDE_CONFIG_DIR temp dir,
// stdin/stdout pipes) lives inside Run. New("") falls back to the `claude`
// binary on $PATH (or $ANVIL_CLAUDE_BIN if set, used by smoke tests).
type Adapter struct {
	binPath string
}

// New constructs an Adapter. If binPath is empty, $ANVIL_CLAUDE_BIN is
// consulted, then `claude` on $PATH at Run time.
func New(binPath string) *Adapter { return &Adapter{binPath: binPath} }

// Name returns the adapter identifier persisted in telemetry.
func (a *Adapter) Name() string { return "claude-code" }

// Run spawns the Claude CLI for one task. Per spec §1: isolated
// CLAUDE_CONFIG_DIR, 8 MiB stdout scanner, cmd.Cancel + cmd.WaitDelay on
// ctx-cancel, NDJSON parsing of usage/cost, quota-message detection.
func (a *Adapter) Run(ctx context.Context, req build.RunRequest) (build.RunResult, error) {
	bin, err := a.resolveBin()
	if err != nil {
		return build.RunResult{}, err
	}

	cfgDir, err := os.MkdirTemp("", "anvil-claude-cfg-")
	if err != nil {
		return build.RunResult{}, fmt.Errorf("creating CLAUDE_CONFIG_DIR: %w", err)
	}
	defer os.RemoveAll(cfgDir)

	if err := writeSettings(cfgDir, req); err != nil {
		return build.RunResult{}, fmt.Errorf("writing settings.json: %w", err)
	}

	runCtx := ctx
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--model", req.Model,
	}
	if req.Cwd != "" {
		args = append(args, "--add-dir", req.Cwd)
	}

	cmd := exec.CommandContext(runCtx, bin, args...)
	cmd.Dir = req.Cwd
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+cfgDir)
	// Per go-conventions.md: cmd.Cancel + WaitDelay for graceful-then-forceful
	// shutdown when ctx is cancelled.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return build.RunResult{}, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return build.RunResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return build.RunResult{}, fmt.Errorf("stderr pipe: %w", err)
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return build.RunResult{}, fmt.Errorf("starting claude: %w", err)
	}

	// Pipe the prompt + context-file references on stdin, then close so the
	// CLI sees EOF.
	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin, buildPrompt(req))
	}()

	// Drain stderr to a buffer for diagnostics; quota detection doesn't
	// depend on it (the result event carries the message), but it surfaces
	// useful errors when the spawn itself fails.
	var stderrBuf strings.Builder
	go func() { _, _ = io.Copy(&stderrBuf, stderr) }()

	res, quotaSeen := scanStdout(stdout)

	waitErr := cmd.Wait()
	res.Duration = time.Since(start)
	if exitErr := (*exec.ExitError)(nil); errors.As(waitErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	} else if waitErr != nil {
		// Process never started cleanly OR pipe error; surface it so build
		// classifies as failed rather than success.
		return res, fmt.Errorf("claude wait: %w (stderr: %s)", waitErr, strings.TrimSpace(stderrBuf.String()))
	}

	if quotaSeen {
		return res, build.ErrQuotaExhausted
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, ctxErr
	}
	return res, nil
}

// resolveBin picks the binary path: explicit field > $ANVIL_CLAUDE_BIN >
// `claude` on $PATH.
func (a *Adapter) resolveBin() (string, error) {
	if a.binPath != "" {
		return a.binPath, nil
	}
	if env := os.Getenv("ANVIL_CLAUDE_BIN"); env != "" {
		return env, nil
	}
	bin, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude binary not found on $PATH and $ANVIL_CLAUDE_BIN unset: %w", err)
	}
	return bin, nil
}

// writeSettings places the per-spawn settings.json into CLAUDE_CONFIG_DIR.
// Carries the thinking-budget tier (translated from req.Effort) and the
// allow-list of skills the agent may load. Plain context files are surfaced
// in the prompt body (see buildPrompt), not here.
func writeSettings(cfgDir string, req build.RunRequest) error {
	type extended struct {
		Budget string `json:"budget"`
	}
	type skills struct {
		Allow []string `json:"allow,omitempty"`
	}
	settings := struct {
		ExtendedThinking extended `json:"extendedThinking"`
		Skills           skills   `json:"skills,omitempty"`
	}{
		ExtendedThinking: extended{Budget: translateEffort(req.Effort)},
		Skills:           skills{Allow: req.Skills},
	}
	b, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfgDir, "settings.json"), b, 0o600)
}

// buildPrompt prepends a "## Context files" block (if any) to req.Instruction.
// Skills go through settings.json, not the prompt body.
func buildPrompt(req build.RunRequest) string {
	if len(req.Context) == 0 {
		return req.Instruction
	}
	var b strings.Builder
	b.WriteString("## Context files\n")
	for _, c := range req.Context {
		b.WriteString("- ")
		b.WriteString(c)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(req.Instruction)
	return b.String()
}

// scanStdout reads NDJSON from r with the 8 MiB scanner buffer required by
// go-conventions.md:49. Returns the aggregated RunResult and whether any
// event surfaced a usage-cap message.
func scanStdout(r io.Reader) (build.RunResult, bool) {
	var res build.RunResult
	var quotaSeen bool

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		ev, ok := parseEvent(sc.Bytes())
		if !ok {
			continue
		}
		if ev.Text != "" && isQuotaMessage(ev.Text) {
			quotaSeen = true
		}
		if ev.Type == "result" {
			res.Tokens.Input = ev.Usage.InputTokens
			res.Tokens.Output = ev.Usage.OutputTokens
			res.Tokens.CacheRead = ev.Usage.CacheRead
			res.Tokens.CacheWrite = ev.Usage.CacheCreate
			res.CostUSD = ev.CostUSD
			res.AgentTime = time.Duration(ev.DurationMS) * time.Millisecond
		}
	}
	return res, quotaSeen
}
