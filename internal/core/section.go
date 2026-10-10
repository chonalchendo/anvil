package core

import (
	"strings"
)

// fenceOpen reports the fence character and length when line opens or closes
// a fenced code block (up to three spaces of indent, then 3+ backticks or
// tildes).
func fenceOpen(line string) (byte, int) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0
	}
	c := trimmed[0]
	n := len(trimmed) - len(strings.TrimLeft(trimmed, string(c)))
	if n < 3 {
		return 0, 0
	}
	if c == '`' && strings.Contains(trimmed[n:], "`") {
		return 0, 0
	}
	return c, n
}

// atxHeading returns the level and trimmed text of an ATX heading line, or
// level 0 when line is not one.
func atxHeading(line string) (int, string) {
	n := len(line) - len(strings.TrimLeft(line, "#"))
	if n == 0 || n > 6 || len(line) == n || (line[n] != ' ' && line[n] != '\t') {
		return 0, ""
	}
	return n, strings.Join(strings.Fields(line[n:]), " ")
}

// ScanSection finds the section whose heading is want: a full ATX heading such
// as "### Sub", or a bare name read as an H2. It returns the section text
// (heading line through the line before the next heading of the same or a
// shallower level, trailing blank lines trimmed) and the H2 headings of body,
// in order. found is false when want is absent; text is then "".
// Fenced code blocks never open or close a section: the fence tracks its
// opening character and length, so ~~~ fences and a 4-backtick fence quoting a
// 3-backtick sample are both honoured.
func ScanSection(body, want string) (text string, h2s []string, found bool) {
	want = strings.TrimSpace(want)
	if !strings.HasPrefix(want, "#") {
		want = "## " + want
	}
	wantLevel, wantText := atxHeading(want)
	lines := strings.Split(body, "\n")
	start, level := -1, 0
	var fenceChar byte
	fenceLen := 0
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if fenceChar != 0 {
			if c, n := fenceOpen(line); c == fenceChar && n >= fenceLen && strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(line, " "), string(c))) == "" {
				fenceChar, fenceLen = 0, 0
			}
			continue
		}
		if c, n := fenceOpen(line); c != 0 {
			fenceChar, fenceLen = c, n
			continue
		}
		l, txt := atxHeading(line)
		if l == 0 {
			continue
		}
		if start >= 0 {
			if l <= level {
				return strings.TrimSpace(strings.Join(lines[start:i], "\n")), h2s, true
			}
			continue
		}
		if l == 2 {
			h2s = append(h2s, "## "+txt)
		}
		if l == wantLevel && txt == wantText {
			start, level = i, l
		}
	}
	if start < 0 {
		return "", h2s, false
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n")), h2s, true
}

// Section returns the body text under a `## <heading>` line, without the
// heading, up to the next heading of level 2 or shallower (or EOF). Returns ""
// when the heading is absent. Fenced code blocks are skipped (see
// ScanSection), so a heading quoted in a code sample neither opens nor closes
// the section. A duplicate heading is malformed input (RequiredIssueSections
// rejects it): the first occurrence wins.
// Shared by index.TLDRSection (`## TL;DR`, scanned as-is so a fenced example
// inside the digest survives verbatim) and BodyLinksSectionTargets
// (`## Links`, called on StripFencedBlocks(body) so an illustrative wikilink
// inside a code sample is excluded from the section text itself, not just
// from heading detection — the two callers deliberately differ here).
func Section(body, heading string) string {
	text, _, ok := ScanSection(body, "## "+heading)
	if !ok {
		return ""
	}
	_, rest, _ := strings.Cut(text, "\n")
	return strings.TrimSpace(rest)
}
