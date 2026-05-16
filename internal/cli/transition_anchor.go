package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// anchorTimeout caps a single anchor command. The gate fires synchronously at
// claim time so the agent gets fast feedback; commands that take longer should
// be slimmed in the issue's frontmatter.
const anchorTimeout = 30 * time.Second

// shaRe matches an `expected` field that opts into sha256-digest comparison.
var shaRe = regexp.MustCompile(`(?i)^sha:[0-9a-f]+$`)

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

	c := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
	var stdout bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = stderr
	// Exit code is ignored: a non-zero exit with mismatched stdout still
	// surfaces as a mismatch; a non-zero exit whose stdout happens to match
	// the expected value is treated as a match (the recorded reproduction
	// includes the exit). This keeps the gate purely about output identity.
	_ = c.Run()

	got := stdout.String()
	if shaRe.MatchString(expected) {
		sum := sha256.Sum256(stdout.Bytes())
		gotDigest := "sha:" + hex.EncodeToString(sum[:])
		if equalFold(gotDigest, expected) {
			return true, cmdStr, "", nil
		}
		return false, cmdStr, fmt.Sprintf("--- expected\n+++ actual (sha256)\n-%s\n+%s\n", expected, gotDigest), nil
	}
	if got == expected {
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
