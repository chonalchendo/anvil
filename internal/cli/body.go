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

// stdinIsPipe reports whether os.Stdin is something other than a character
// device (i.e. a pipe or redirected file). Uses Stat mode bits — no new dep.
func stdinIsPipe() (bool, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	return (fi.Mode() & os.ModeCharDevice) == 0, nil
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
