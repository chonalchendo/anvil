package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// anchorTimeout caps a single anchor command. The gate fires synchronously at
// claim time so the agent gets fast feedback; commands that take longer should
// be slimmed in the issue's frontmatter.
const anchorTimeout = 30 * time.Second

// anchorMaxStdoutBytes caps captured stdout to prevent runaway memory growth
// from a noisy anchor command (e.g. an accidental `yes` or unbounded log
// stream). Real anchors emit a line or a few KB; 256 KiB is generous slack.
const anchorMaxStdoutBytes = 256 * 1024

// shaRe matches an `expected` field that opts into sha256-digest comparison.
var shaRe = regexp.MustCompile(`(?i)^sha:[0-9a-f]+$`)

// ansiRe strips ANSI/VT100 escape sequences (CSI sequences and OSC sequences)
// emitted by progress bars, colour codes, and cursor-control sequences.
var ansiRe = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[A-Za-z]|\][^\x07]*(?:\x07|\x1b\\))`)

// normalizeAnchorOutput renders terminal control sequences so that progress-bar
// noise (carriage-return overwrites, ANSI escapes) does not pollute the
// comparison. The algorithm:
//  1. Strip ANSI/VT100 escape sequences.
//  2. Split on \n (physical newlines). Drop any segment that begins with \r —
//     these are pure carriage-return progress-overwrite lines (the pattern used
//     by DuckDB and similar engines to redraw a progress bar in-place). Such
//     lines carry no payload; their only effect on a real terminal is to redraw
//     column 0, which leaves no residual text once the next \n-delimited line
//     prints the actual value.
//  3. Rejoin surviving segments with \n.
//
// Trailing \n is not stripped here; callers that need trailing-newline tolerance
// apply strings.TrimSuffix independently.
func normalizeAnchorOutput(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, line := range lines {
		// Lines starting with \r are pure progress-redraw lines; discard them.
		if len(line) > 0 && line[0] == '\r' {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// capWriter writes to an underlying buffer up to a fixed cap, then silently
// discards further bytes (still claiming acceptance so the producer's pipe
// doesn't block). Truncation is exposed via Truncated.
type capWriter struct {
	buf       bytes.Buffer
	cap       int
	truncated bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	remaining := w.cap - w.buf.Len()
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		w.buf.Write(p[:remaining])
		w.truncated = true
		return len(p), nil
	}
	return w.buf.Write(p)
}

// runAnchorCheck runs the issue's reproduction_anchor (if any) and reports
// whether the observed output matches the recorded `expected` value. Returns
// matched=true with diff="" when the issue carries no anchor (grandfather rule).
//
// stderr is streamed through `stderr` so the agent sees what the command
// printed; stdout is captured for comparison.
func runAnchorCheck(ctx context.Context, a *core.Artifact, stderr io.Writer) (matched bool, command, diff string, err error) {
	anchor, ok := a.FrontMatter["reproduction_anchor"].(map[string]any)
	if !ok {
		return true, "", "", nil
	}
	cmdStr, _ := anchor["command"].(string)
	expected, _ := anchor["expected"].(string)
	if cmdStr == "" {
		return true, "", "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, anchorTimeout)
	defer cancel()

	c := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr) //nolint:gosec // G204: runs reproduction_anchor.command verbatim by design; anchors are author-trusted vault content, bounded by anchorTimeout
	stdout := &capWriter{cap: anchorMaxStdoutBytes}
	c.Stdout = stdout
	c.Stderr = stderr
	// Exit code is ignored for ExitError: a non-zero exit with mismatched
	// stdout still surfaces as a mismatch; a non-zero exit whose stdout
	// happens to match the expected value is treated as a match (the
	// recorded reproduction includes the exit). Timeout and exec-startup
	// failures are surfaced as hard errors — they don't carry meaningful
	// "output identity" semantics.
	runErr := c.Run()
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(runErr, &exitErr):
			// fall through to output comparison
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return false, cmdStr, "", fmt.Errorf("anchor command timed out after %s", anchorTimeout)
		default:
			return false, cmdStr, "", fmt.Errorf("anchor command failed: %w", runErr)
		}
	}
	if stdout.truncated {
		return false, cmdStr, "", fmt.Errorf("anchor stdout exceeded %d bytes", anchorMaxStdoutBytes)
	}

	got := normalizeAnchorOutput(stdout.buf.String())
	if shaRe.MatchString(expected) {
		sum := sha256.Sum256(stdout.buf.Bytes())
		gotDigest := "sha:" + hex.EncodeToString(sum[:])
		if equalFold(gotDigest, expected) {
			return true, cmdStr, "", nil
		}
		return false, cmdStr, fmt.Sprintf("--- expected\n+++ actual (sha256)\n-%s\n+%s\n", expected, gotDigest), nil
	}
	// Strip at most one trailing newline from each side: echo-based commands
	// always append \n but authors record the bare string in --expected.
	if strings.TrimSuffix(got, "\n") == strings.TrimSuffix(expected, "\n") {
		return true, cmdStr, "", nil
	}
	return false, cmdStr, fmt.Sprintf("--- expected\n+++ actual\n-%q\n+%q\n", expected, got), nil
}

// equalFold compares two ASCII strings case-insensitively. Hex digits are
// ASCII-only so a manual fold avoids dragging in strings/unicode just for the
// sha-prefix match.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
