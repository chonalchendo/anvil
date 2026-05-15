package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// readBody resolves the body content for `create` and `inbox add`. Sources
// are mutually exclusive — supplying more than one is an error so the agent
// learns which input the CLI used:
//
//   - `--body-file <path>`  → file contents.
//   - `--body -` (explicit) → read all of stdin; requires piped stdin.
//   - `--body <literal>`    → literal string.
//   - piped stdin alone     → read all of stdin (legacy form).
//   - none of the above     → "".
//
// A trailing newline is ensured on non-empty bodies so the artifact file
// always terminates cleanly.
func readBody(_ *cobra.Command, flagBody, flagBodyFile string) (string, error) {
	piped, err := stdinIsPipe()
	if err != nil {
		return "", fmt.Errorf("stat stdin: %w", err)
	}

	stdinFlag := flagBody == "-"
	bodyLiteral := flagBody != "" && !stdinFlag
	hasFile := flagBodyFile != ""

	// piped stdin combined with --body - is the canonical "read from stdin"
	// pair; both refer to the same source. Any other combination is ambiguous.
	switch {
	case bodyLiteral && hasFile,
		bodyLiteral && piped,
		hasFile && piped,
		hasFile && stdinFlag:
		if piped {
			_, _ = io.Copy(io.Discard, os.Stdin)
		}
		return "", errors.New("--body, --body-file, and piped stdin are mutually exclusive")
	}

	if hasFile {
		b, err := os.ReadFile(flagBodyFile)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", flagBodyFile, err)
		}
		if len(b) == 0 {
			return "", nil
		}
		return ensureTrailingNewline(string(b)), nil
	}

	if stdinFlag && !piped {
		return "", errors.New("--body - requires piped stdin")
	}

	if stdinFlag || piped {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if len(b) == 0 {
			return "", nil
		}
		return ensureTrailingNewline(string(b)), nil
	}

	if bodyLiteral {
		return ensureTrailingNewline(flagBody), nil
	}

	return "", nil
}

// stdinIsPipe reports whether os.Stdin carries real piped/redirected data
// the caller intends us to read. Only two shapes qualify:
//
//   - a named pipe (`echo x | cmd`, heredocs, process substitution)
//   - a regular file with non-zero size (`cmd < file.md`)
//
// Sockets, ttys, /dev/null, and empty files all return false so we never
// block on a read that has no producer — e.g. when an agent harness
// attaches a persistent unix socket to stdin without ever writing to it.
func stdinIsPipe() (bool, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	m := fi.Mode()
	if m&os.ModeNamedPipe != 0 {
		return true, nil
	}
	if m.IsRegular() && fi.Size() > 0 {
		return true, nil
	}
	return false, nil
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
