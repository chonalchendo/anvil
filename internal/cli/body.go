package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// readBody resolves the body content for `create` and `inbox add`:
//   - flagBody set, stdin tty → use flagBody.
//   - flagBody empty, stdin non-tty → read all of stdin.
//   - both → error (silent discard would be a footgun).
//   - neither → "".
//
// A trailing newline is ensured on non-empty bodies so the artifact file
// always terminates cleanly.
func readBody(_ *cobra.Command, flagBody string) (string, error) {
	piped, err := stdinIsPipe()
	if err != nil {
		return "", fmt.Errorf("stat stdin: %w", err)
	}

	if flagBody != "" && piped {
		_, _ = io.Copy(io.Discard, os.Stdin)
		return "", errors.New("--body and piped stdin are mutually exclusive")
	}

	if flagBody != "" {
		return ensureTrailingNewline(flagBody), nil
	}

	if !piped {
		return "", nil
	}

	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	if len(b) == 0 {
		return "", nil
	}
	return ensureTrailingNewline(string(b)), nil
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
