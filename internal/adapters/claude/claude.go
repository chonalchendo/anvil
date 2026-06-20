package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// Adapter spawns the Claude Code CLI per task. One Adapter is shared across
// every task in a build — the per-spawn state (settings JSON, stdin/stdout
// pipes) lives inside Run. New("") falls back to the `claude` binary on
// $PATH (or $ANVIL_CLAUDE_BIN if set, used by smoke tests).
type Adapter struct {
	binPath string
}

// New constructs an Adapter. If binPath is empty, $ANVIL_CLAUDE_BIN is
// consulted, then `claude` on $PATH at Run time.
func New(binPath string) *Adapter { return &Adapter{binPath: binPath} }

// Name returns the adapter identifier persisted in telemetry.
func (a *Adapter) Name() string { return "claude-code" }

// Run spawns the Claude CLI for one task. Per spec §1: 8 MiB stdout scanner,
// cmd.Cancel + cmd.WaitDelay on ctx-cancel, NDJSON parsing of usage/cost,
// quota-message detection. Settings (thinking budget, skills allow-list) are
// layered via --settings argv.
//
// Isolation + seeding (adapters.md "per-spawn state isolation is invariant"):
// each Run mints a fresh CLAUDE_CONFIG_DIR so parallel tasks cannot clobber each
// other's session/auth state, then SEEDS the user's credentials into it before
// spawn. A bare empty config dir has no .credentials.json, so `claude -p`
// returns "Not logged in" and does no work (commit 084f9f7). The dir is removed
// after Wait. On macOS the OAuth token lives in the login Keychain rather than
// on disk; seedConfigDir pulls it from there so the isolated spawn authenticates
// (anvil.0107).
func (a *Adapter) Run(ctx context.Context, req build.RunRequest) (build.RunResult, error) {
	bin, err := a.resolveBin()
	if err != nil {
		return build.RunResult{}, err
	}

	settingsArg, err := settingsJSON(req)
	if err != nil {
		return build.RunResult{}, fmt.Errorf("building settings JSON: %w", err)
	}

	// Isolate each spawn so parallel tasks cannot clobber each other's
	// session/auth state. The dir is removed after the process exits.
	configDir, err := os.MkdirTemp("", "anvil-claude-*")
	if err != nil {
		return build.RunResult{}, fmt.Errorf("creating per-spawn config dir: %w", err)
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // cleanup; not load-bearing

	// Seed credentials into the fresh dir BEFORE spawn — otherwise the empty
	// dir strips auth and the spawn no-ops with "Not logged in".
	if err := seedConfigDir(configDir); err != nil {
		return build.RunResult{}, fmt.Errorf("seeding per-spawn config dir: %w", err)
	}

	runCtx := ctx
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	// why: anvil-builds are autonomous-by-design; the user opts in by invoking
	// `anvil build`. Without bypassPermissions, claude --print auto-denies
	// any tool that requires user approval (Write, Edit, Bash), silently
	// no-oping most real engineering work. A plan-task-level override is
	// reasonable but out of v0.1 scope.
	args := []string{
		"--settings", settingsArg,
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "bypassPermissions",
		"--model", req.Model,
	}
	if req.Cwd != "" {
		args = append(args, "--add-dir", req.Cwd)
	}

	cmd := exec.CommandContext(runCtx, bin, args...) //nolint:gosec // bin resolved from trusted sources: explicit field, $ANVIL_CLAUDE_BIN, or PATH lookup
	cmd.Dir = req.Cwd
	// Point the child at the seeded per-spawn config dir. Using os.Environ() as
	// the base ensures the child inherits PATH, HOME, and any credential
	// env-vars the user set, while overriding CLAUDE_CONFIG_DIR for isolation.
	// Auth mode is subscription (no ANTHROPIC_API_KEY override).
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+configDir)
	// Setpgid puts the child and all its descendants in a new process group so
	// cancellation kills the whole tree (e.g. a `sleep` spawned by the shim),
	// not just the top-level shell. Per go-conventions.md: cmd.Cancel +
	// WaitDelay for graceful-then-forceful shutdown when ctx is cancelled.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}
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

	// Claude Code reads the prompt from stdin and expects EOF to begin processing.
	go func() {
		defer stdin.Close() //nolint:errcheck // close in goroutine; error not actionable after write
		_, _ = io.WriteString(stdin, buildPrompt(req))
	}()

	// Drain stderr to a buffer for diagnostics; quota detection doesn't
	// depend on it (the result event carries the message), but it surfaces
	// useful errors when the spawn itself fails. stderrDone is closed when
	// the goroutine finishes so the post-Wait read of stderrBuf is race-free.
	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		_, _ = io.Copy(&stderrBuf, stderr)
	}()

	res, quotaSeen, lastText := scanStdout(stdout)

	waitErr := cmd.Wait()
	res.Duration = time.Since(start)
	<-stderrDone // goroutine finished; stderrBuf safe to read

	if lastText != "" {
		res.Diagnostic = lastText
	} else if s := strings.TrimSpace(stderrBuf.String()); s != "" {
		res.Diagnostic = s
	}

	if exitErr := (*exec.ExitError)(nil); errors.As(waitErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	} else if waitErr != nil {
		// Process never started cleanly OR pipe error; surface it so build
		// classifies as failed rather than success.
		return res, fmt.Errorf("claude wait: %w (stderr: %s)", waitErr, strings.TrimSpace(stderrBuf.String()))
	}

	// Record the isolation metadata regardless of outcome so callers and
	// telemetry can correlate logs. configDir is already being removed by
	// the deferred RemoveAll, but the path is still meaningful at return time.
	res.ConfigDir = configDir
	res.AuthMode = "subscription" // no ANTHROPIC_API_KEY override; draws subscription limits

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

// seedConfigDir copies the user's auth/session state from their real Claude
// config dir into the fresh per-spawn dir, so the isolated `claude -p` can still
// authenticate (adapters.md: "credentials seeded in before spawn"). Source is
// $CLAUDE_CONFIG_DIR if set, else ~/.claude — mirroring the install command's
// resolution. When no file-based credential is present — the macOS case, where
// the OAuth token lives in the login Keychain rather than on disk — the token is
// pulled from the Keychain and written into the per-spawn dir (anvil.0107).
func seedConfigDir(dst string) error {
	src, err := sourceConfigDir()
	if err != nil {
		return err
	}
	// .credentials.json is the file-based OAuth/API token (Linux, and macOS
	// when not Keychain-backed). The user-settings file carries config the
	// spawn inherits. Copy each only if present.
	seededCreds := false
	for _, name := range []string{".credentials.json", "settings.json"} {
		from := filepath.Join(src, name)
		b, err := os.ReadFile(from) //nolint:gosec // path built from the user's own config dir
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", from, err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), b, 0o600); err != nil { //nolint:gosec // dst is our own os.MkdirTemp dir; name is a hardcoded literal from a closed list, not untrusted input
			return fmt.Errorf("writing %s: %w", name, err)
		}
		if name == ".credentials.json" {
			seededCreds = true
		}
	}
	// On macOS the OAuth token lives in the login Keychain, so the loop above
	// seeds no credential and the isolated spawn would no-op with "Not logged
	// in". Pull it from the Keychain into the per-spawn dir. Absent (API-key
	// mode or logged out) → skip; the spawn surfaces its own auth error.
	if !seededCreds {
		if blob, ok := keychainCredential(); ok {
			if err := os.WriteFile(filepath.Join(dst, ".credentials.json"), blob, 0o600); err != nil {
				return fmt.Errorf("writing keychain credential: %w", err)
			}
		}
	}
	return nil
}

// keychainService is the service name Claude Code stores its OAuth credential
// under in the macOS login Keychain (account = the login user).
const keychainService = "Claude Code-credentials"

// keychainCredential returns the Claude Code OAuth blob from the macOS login
// Keychain and true when present. On non-macOS hosts, or when the item is absent
// (API-key mode or logged out), it returns false — not an error, the spawn
// surfaces its own auth failure. A package var so tests stub the security(1)
// shell-out, which can otherwise block on a GUI access prompt.
var keychainCredential = func() ([]byte, bool) {
	if runtime.GOOS != "darwin" {
		return nil, false
	}
	out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-w").Output()
	if err != nil {
		return nil, false
	}
	return bytes.TrimSpace(out), true
}

// sourceConfigDir resolves the user's real Claude config dir: $CLAUDE_CONFIG_DIR
// if set, else ~/.claude.
func sourceConfigDir() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// settingsJSON marshals the per-spawn settings to a JSON string for
// --settings argv. Carries the thinking-budget tier (translated from
// req.Effort) and the allow-list of skills the agent may load.
func settingsJSON(req build.RunRequest) (string, error) {
	type extended struct {
		Budget string `json:"budget"`
	}
	type skills struct {
		Allow []string `json:"allow,omitempty"`
	}
	// TODO(integration): verify key names against claude --help / release
	// notes — silent settings drift would be hard to diagnose.
	settings := struct {
		ExtendedThinking extended `json:"extendedThinking"`
		Skills           skills   `json:"skills,omitempty"`
	}{
		ExtendedThinking: extended{Budget: translateEffort(req.Effort)},
		Skills:           skills{Allow: req.Skills},
	}
	b, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	return string(b), nil
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
// go-conventions.md:49. Returns the aggregated RunResult, whether any event
// surfaced a usage-cap message, and the last result-event text (empty if
// none arrived).
func scanStdout(r io.Reader) (build.RunResult, bool, string) {
	var res build.RunResult
	var quotaSeen bool
	var lastText string

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
			lastText = ev.Text
		}
	}
	return res, quotaSeen, lastText
}
