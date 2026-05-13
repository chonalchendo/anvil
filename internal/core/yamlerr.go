package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// yamlLineRe captures the leading "line N: " segment yaml.v3 emits on parser
// errors with a known position. The remainder of the message is the symptom
// text (e.g. "did not find expected key").
var yamlLineRe = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

// enrichYAMLError wraps a yaml.v3 parse error from the frontmatter bytes with
// the offending line excerpt, a fix hint when a common quoting/escaping
// pattern is detected, and the frontmatter field the failure landed in when
// resolvable from the lines above.
//
// fmBytes is the frontmatter region (lines between the `---` delimiters); the
// 1-based line number yaml.v3 reports indexes into fmBytes, so we offset by 1
// when surfacing the file-relative line (the opening `---` is file line 1).
func enrichYAMLError(fmBytes []byte, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	yamlLine, symptom, ok := parseYAMLLine(msg)
	lines := strings.Split(string(fmBytes), "\n")

	// Without a line number we still try to locate the first plausibly-broken
	// line by scanning; if nothing trips, return the original error unchanged.
	if !ok {
		idx, hint := scanForCulprit(lines)
		if idx < 0 {
			return err
		}
		return formatEnriched(err, idx+2, lines[idx], "", hint)
	}

	// yaml.v3 reports a symptom line, but the real cause may live on a
	// different line (a stray quote earlier in the document poisons parsing
	// downstream, or the parser recovers and flags the next line). Prefer
	// the first line in the whole frontmatter that classifies as a known
	// bad pattern; fall back to the reported line.
	idx := clamp(yamlLine-1, 0, len(lines)-1)
	hint := classifyLine(lines[idx])
	if hint == "" {
		if culprit, culpritHint := scanForCulprit(lines); culprit >= 0 {
			idx, hint = culprit, culpritHint
		}
	}
	if hint == "" {
		hint = symptomHint(symptom)
	}
	field := resolveField(lines, idx)
	return formatEnriched(err, idx+2, lines[idx], field, hint)
}

func parseYAMLLine(msg string) (int, string, bool) {
	m := yamlLineRe.FindStringSubmatch(msg)
	if m == nil {
		return 0, "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, "", false
	}
	return n, m[2], true
}

// classifyLine returns a fix hint when the line trips a recognised common
// failure pattern. Empty string means "no specific suggestion".
func classifyLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	// Pull the value half of "key: value" (or list-item "- value").
	value := valuePart(trimmed)
	if value == "" {
		return ""
	}

	// Unbalanced double-quote: starts with " but contains an inner unescaped
	// " before the close, or count of " is odd.
	if strings.Count(value, `"`) >= 2 && strings.HasPrefix(value, `"`) && !endsBalancedDQ(value) {
		return "value looks like it has an unescaped double-quote inside a double-quoted string; escape it as \\\" or switch to a literal block scalar (key: |)"
	}
	if strings.Count(value, `"`)%2 == 1 {
		return "unbalanced double-quote in value; quote the whole value with \" and escape inner quotes as \\\""
	}

	// Colon-space inside an unquoted value: "title: foo: bar" — the second
	// ": " makes yaml treat foo as a key.
	if !quotedValue(value) && strings.Contains(value, ": ") {
		return "value contains \": \" which YAML reads as a nested key; quote the value with \" or use a literal block scalar (key: |)"
	}

	// Leading reserved indicators that need quoting: ` @ % * & ! | > ?
	if len(value) > 0 && strings.ContainsRune("`@%*&!|>?", rune(value[0])) && !quotedValue(value) {
		return fmt.Sprintf("value begins with reserved character %q; wrap the value in double quotes", value[0])
	}

	return ""
}

// symptomHint maps a yaml.v3 symptom string to a generic suggestion when no
// line-level pattern matched.
func symptomHint(symptom string) string {
	switch {
	case strings.Contains(symptom, "did not find expected key"),
		strings.Contains(symptom, "did not find expected '-' indicator"):
		return "look for an unescaped \" or a stray : in a value on or above this line; quote the value or use a literal block scalar (key: |)"
	case strings.Contains(symptom, "mapping values are not allowed"):
		return "a value contains \": \" which YAML reads as a nested key; quote the value with \""
	case strings.Contains(symptom, "found character that cannot start any token"):
		return "value begins with a reserved character (one of ` @ % * & ! | > ?); wrap the value in double quotes"
	}
	return ""
}

// scanForCulprit is a fallback for line-less yaml errors: returns the first
// line that classifyLine flags and its hint.
func scanForCulprit(lines []string) (int, string) {
	for i, l := range lines {
		if h := classifyLine(l); h != "" {
			return i, h
		}
	}
	return -1, ""
}

// resolveField walks back from idx to find the enclosing top-level key, e.g.
// the field this line belongs to. Returns "" if none is resolvable.
func resolveField(lines []string, idx int) string {
	// If this line is itself "key: ..." at column 0, that's the field.
	if k := topLevelKey(lines[idx]); k != "" {
		return k
	}
	for i := idx - 1; i >= 0; i-- {
		if k := topLevelKey(lines[i]); k != "" {
			return k
		}
	}
	return ""
}

func topLevelKey(line string) string {
	if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' || line[0] == '#' {
		return ""
	}
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return ""
	}
	return line[:i]
}

// valuePart returns the value portion of a "key: value" or "- value" line,
// or the raw trimmed line if it isn't a key/list-item shape.
func valuePart(trimmed string) string {
	if strings.HasPrefix(trimmed, "- ") {
		return strings.TrimSpace(trimmed[2:])
	}
	i := strings.IndexByte(trimmed, ':')
	if i < 0 {
		return trimmed
	}
	return strings.TrimSpace(trimmed[i+1:])
}

func quotedValue(value string) bool {
	if len(value) < 2 {
		return false
	}
	return (value[0] == '"' && endsBalancedDQ(value)) ||
		(value[0] == '\'' && value[len(value)-1] == '\'')
}

// endsBalancedDQ reports whether a double-quoted scalar is well-formed: opens
// with ", closes with ", and every inner " is preceded by a backslash.
func endsBalancedDQ(value string) bool {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return false
	}
	inner := value[1 : len(value)-1]
	for i := 0; i < len(inner); i++ {
		if inner[i] == '"' && (i == 0 || inner[i-1] != '\\') {
			return false
		}
	}
	return true
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// formatEnriched composes the wrapped error message. fileLine is 1-based and
// counted from the artifact's first byte (i.e. the opening `---` is line 1).
func formatEnriched(orig error, fileLine int, lineText, field, hint string) error {
	var b strings.Builder
	b.WriteString(orig.Error())
	b.WriteString("\n  near line ")
	b.WriteString(strconv.Itoa(fileLine))
	if field != "" {
		b.WriteString(" (field: ")
		b.WriteString(field)
		b.WriteString(")")
	}
	b.WriteString(": ")
	b.WriteString(strings.TrimRight(lineText, " \t"))
	if hint != "" {
		b.WriteString("\n  hint: ")
		b.WriteString(hint)
	}
	return fmt.Errorf("%s", b.String())
}
