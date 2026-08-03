package cli

import (
	"fmt"
	"path"
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

// validateDeclaredGlobs rejects a declared entry carrying internal whitespace:
// no glob syntax contains a space, so a whitespace-bearing entry is prose (a
// dispatch prompt's file estimate, not a real path) that would silently match
// nothing and false-flag every changed file as out-of-scope. Reject it loudly,
// naming the entry, at the one choke-point both the CLI and any future caller
// pass through — rather than let it degrade into a zero-match scope report.
func validateDeclaredGlobs(declared []string) error {
	for _, d := range declared {
		if strings.ContainsAny(d, " \t\n") {
			return fmt.Errorf("declared entry %q contains whitespace, so it cannot be a glob; declare a real path or pattern (*, ?, {a,b}), not a prose file estimate", d)
		}
	}
	return nil
}

// matchesAny reports whether f matches any brace-free glob pattern, either as a
// glob or as a directory f sits beneath. A malformed pattern falls back to
// literal comparison rather than silently covering nothing.
func matchesAny(patterns []string, f string) bool {
	// The prefix test below is byte-level, so an uncleaned `pkg/../../etc/shadow`
	// reads as sitting beneath a declared `pkg` and escapes the gate. Clean for
	// that test only: the exact and glob comparisons must keep seeing the path
	// exactly as `git diff --name-only` reported it.
	cleaned := path.Clean(f)
	for _, p := range patterns {
		if p == f {
			return true
		}
		if ok, err := filepath.Match(p, f); err == nil && ok {
			return true
		}
		if d := dirPrefix(p); d != "" && strings.HasPrefix(cleaned, d) {
			return true
		}
	}
	return false
}

// dirPrefix returns the "everything beneath here" prefix a declared entry
// denotes. Dispatch prompts write the directory both ways — `pkg/` and a bare
// package root `pkg` — and a dotted final segment is no evidence of a file
// (`.github` is a directory), so every non-empty entry gets a prefix. Brace
// expansion can yield an empty pattern (`{,pkg}`), which must cover nothing.
// The prefix always ends in `/`, so a prefix excludes siblings rather than
// substrings: `pkg` cannot swallow `pkgtools/x.go`, nor `a/b.go` `a/b.go.bak`.
func dirPrefix(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasSuffix(p, "/") {
		return p
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
