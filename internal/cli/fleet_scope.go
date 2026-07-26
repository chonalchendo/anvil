package cli

import (
	"path/filepath"
	"strings"
)

// scopeViolations returns the elements of changed that no declared entry covers.
// Declared entries are globs — `*` (any number of them), `?`, and brace
// alternation `{a,b}` — because dispatch prompts routinely carry that notation;
// treating them literally false-flags correctly-scoped files. A declared
// directory covers everything beneath it, so a new file lands in scope.
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

// matchesAny reports whether path matches any brace-free glob pattern, either
// as a glob or as a directory the path sits beneath. A malformed pattern falls
// back to literal comparison rather than silently covering nothing.
func matchesAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if p == path {
			return true
		}
		if ok, err := filepath.Match(p, path); err == nil && ok {
			return true
		}
		if d := dirPrefix(p); d != "" && strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// dirPrefix returns the "everything beneath here" prefix a declared entry
// denotes, or "" when the entry names a file. Dispatch prompts write the
// directory both ways — `pkg/` and a bare package root `pkg` — so a final
// segment carrying no `.` counts as a directory too. The prefix always ends in
// `/`: `pkg` must not swallow `pkgtools/x.go`.
func dirPrefix(p string) string {
	if strings.HasSuffix(p, "/") {
		return p
	}
	if strings.Contains(filepath.Base(p), ".") {
		return ""
	}
	return p + "/"
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

// splitLiteralCSV splits a comma-separated string into trimmed, non-empty
// tokens. Every comma separates, because the tokens name literal paths.
func splitLiteralCSV(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// splitCSV splits a comma-separated string of glob patterns into trimmed,
// non-empty tokens. Commas inside brace alternation
// (`a/met_{growth,health}.sql`) separate alternatives, not tokens, so they do
// not split. An unbalanced `{` degrades the whole string to literal comma
// splitting: a typo must not swallow every following entry and re-arm the
// spurious out-of-scope report this splitting exists to prevent.
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
	if depth != 0 {
		return splitLiteralCSV(s)
	}
	if f := strings.TrimSpace(s[start:]); f != "" {
		out = append(out, f)
	}
	return out
}
