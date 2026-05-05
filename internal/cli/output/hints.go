package output

import (
	"fmt"
	"strings"
)

// TruncationHint returns a one-line stderr hint for a bounded list verb,
// or "" when no truncation occurred. label describes the sort dimension
// ("most recent" / "oldest first"). filters lists narrowing flag names.
func TruncationHint(label string, returned, total int, filters []string) string {
	if returned >= total {
		return ""
	}
	return fmt.Sprintf("showing %d of %d %s; narrow with %s, or raise --limit",
		returned, total, label, strings.Join(filters, ", "))
}

// BodyClipHint formats the stderr hint for a clipped --full body.
func BodyClipHint(returned, total int, path string) string {
	return fmt.Sprintf("body truncated to %d of %d lines; read %s directly for the rest",
		returned, total, path)
}
