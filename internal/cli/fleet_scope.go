package cli

import (
	"path/filepath"
	"strings"
)

// scopeViolations returns the elements of changed that no declared entry covers.
// Declared entries are globs — `*` (any number of them), `?`, and brace
// alternation `{a,b}` — because dispatch prompts routinely carry that notation;
// treating them literally false-flags correctly-scoped files.
func scopeViolations(declared, changed []string) []string {
	var patterns []string
	for _, d := range declared {
		patterns = append(patterns, expandBraces(d)...)
	}
	var outside []string
	for _, f := range changed {
		if !matchesAny(patterns, f) {
			outside = append(outside, f)
		}
	}
	return outside
}

// matchesAny reports whether path matches any brace-free glob pattern. A
// malformed pattern falls back to literal comparison rather than silently
// covering nothing.
func matchesAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if p == path {
			return true
		}
		if ok, err := filepath.Match(p, path); err == nil && ok {
			return true
		}
	}
	return false
}

// expandBraces expands brace alternation into one pattern per alternative:
// `a{b,c}d` -> [abd, acd], nesting included. An unbalanced brace yields the
// pattern unchanged so it still matches literally.
func expandBraces(p string) []string {
	open := strings.IndexByte(p, '{')
	if open < 0 {
		return []string{p}
	}
	var alts []string
	depth, start := 0, open+1
	for i := open; i < len(p); i++ {
		switch p[i] {
		case '{':
			depth++
		case ',':
			if depth == 1 {
				alts = append(alts, p[start:i])
				start = i + 1
			}
		case '}':
			depth--
			if depth == 0 {
				alts = append(alts, p[start:i])
				var out []string
				for _, a := range alts {
					out = append(out, expandBraces(p[:open]+a+p[i+1:])...)
				}
				return out
			}
		}
	}
	return []string{p}
}

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
// Commas inside brace alternation (`a/met_{growth,health}.sql`) separate
// alternatives, not tokens, so they do not split.
func splitCSV(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				if f := strings.TrimSpace(s[start:i]); f != "" {
					out = append(out, f)
				}
				start = i + 1
			}
		}
	}
	if f := strings.TrimSpace(s[start:]); f != "" {
		out = append(out, f)
	}
	return out
}
