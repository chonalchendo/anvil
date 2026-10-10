package ui

import (
	"fmt"
	"iter"
	"strings"
)

// judgeKeys are the frontmatter fields a reader judges a node by, per type.
// Only these types get a strip; the keys leave "All properties".
var judgeKeys = map[string][]string{
	"learning":  {"confidence", "diataxis"},
	"decision":  {"date", "supersedes", "superseded_by"},
	"milestone": {"approved", "done"},
	"issue":     {"verified_verdict", "verified_commit", "verified_at", "external_links", "cost_rounds", "cost_tokens", "cost_diff", "cost_files"},
}

// judge builds the judge strip: set frontmatter fields in judgeKeys order,
// then a milestone's last Measured: line.
func (r resolver) judge(typ string, fm map[string]any, body string) []prop {
	keys, ok := judgeKeys[typ]
	if !ok {
		return nil
	}
	if typ == "issue" {
		return issueJudge(fm)
	}
	var out []prop
	for _, k := range keys {
		if isSet(fm[k]) {
			out = append(out, r.prop(k, fm[k]))
		}
	}
	if typ == "milestone" {
		if m := lastMeasured(body); m != "" {
			out = append(out, prop{Name: "measured", Values: []link{{Text: m, Plain: true}}})
		}
	}
	return out
}

// part is one fragment of a judge value that a plain link cannot carry: a
// status span, a PR link, a <time>, or text with a title.
type part struct {
	Text, Href, Class, Title, ISO string
}

// issueJudge builds an issue's strip from frontmatter only; a missing field
// adds no row.
func issueJudge(fm map[string]any) []prop {
	var out []prop
	if v := fmString(fm["verified_verdict"]); v != "" {
		class, glyph := "status status-done", "✓"
		if v != "pass" {
			class, glyph = "status status-escalated", "▲"
		}
		ps := []part{{Text: glyph + " " + v, Class: class}}
		if c := fmString(fm["verified_commit"]); c != "" {
			ps = append(ps, part{Text: "at"}, part{Text: c[:min(len(c), 7)], Class: "sha"})
		}
		if at := fmString(fm["verified_at"]); at != "" {
			ps = append(ps, part{Text: shortDate(at), ISO: at})
		}
		out = append(out, prop{Name: "Verdict", Rich: ps})
	}
	var prs []part
	links, _ := fm["external_links"].([]any)
	for _, l := range links {
		if u, _ := l.(string); u != "" {
			if n, ok := prNumber(u); ok {
				prs = append(prs, part{Text: "#" + n, Href: u})
			}
		}
	}
	if len(prs) > 0 {
		out = append(out, prop{Name: "PR", Rich: prs})
	}
	if n := count(fm["cost_rounds"]); n != "" {
		out = append(out, prop{Name: "Rounds", Values: []link{{Text: n, Plain: true}}})
	}
	if t := tokens(fm["cost_tokens"]); t != "" {
		exact, _ := num(fm["cost_tokens"])
		out = append(out, prop{Name: "Tokens", Rich: []part{{Text: t, Title: groupDigits(int64(exact))}}})
	}
	if d := count(fm["cost_diff"]); d != "" {
		change := d + " lines"
		if f := count(fm["cost_files"]); f != "" {
			change += " in " + f + " files"
		}
		out = append(out, prop{Name: "Change", Values: []link{{Text: change, Plain: true}}})
	}
	return out
}

// groupDigits writes n with comma thousands separators: 23,313,576.
func groupDigits(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// prNumber returns the number of a GitHub pull URL.
func prNumber(u string) (string, bool) {
	_, rest, ok := strings.Cut(u, "/pull/")
	n, _, _ := strings.Cut(rest, "/")
	return n, ok && n != ""
}

// isSet reports whether a frontmatter value shows: not nil, not blank, not an empty list.
func isSet(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case []any:
		return len(x) > 0
	}
	return strings.TrimSpace(fmt.Sprint(v)) != ""
}

// statusLines yields the lines of the body's "## Status" section.
func statusLines(body string) iter.Seq[string] {
	return func(yield func(string) bool) {
		in := false
		for line := range strings.SplitSeq(body, "\n") {
			if h, ok := strings.CutPrefix(line, "## "); ok {
				in = strings.TrimSpace(h) == "Status"
				continue
			}
			if in && !yield(line) {
				return
			}
		}
	}
}

// lastMeasured returns the last "Measured:" line, label dropped, inside the Status section.
func lastMeasured(body string) string {
	var got string
	for line := range statusLines(body) {
		if rest, ok := strings.CutPrefix(line, "Measured:"); ok {
			got = strings.TrimSpace(strings.ReplaceAll(rest, "`", ""))
		}
	}
	return got
}
