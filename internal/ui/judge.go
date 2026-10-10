package ui

import (
	"fmt"
	"iter"
	"strings"
)

// The issue strip's frontmatter field names. judgeKeys lists them, so the strip and the skip
// from "All properties" cannot drift.
const (
	kVerdict = "verified_verdict"
	kCommit  = "verified_commit"
	kAt      = "verified_at"
	kRounds  = "cost_rounds"
	kTokens  = "cost_tokens"
	kDiff    = "cost_diff"
	kFiles   = "cost_files"
)

// judgeKeys are the frontmatter fields a reader judges a node by, per type. Only these types
// get a strip; the keys leave "All properties". An issue's external_links is not listed: the
// strip shows its pull URLs only, and judgedValue splits them out.
var judgeKeys = map[string][]string{
	"learning":  {"confidence"},
	"decision":  {"supersedes", "superseded_by"},
	"milestone": {"approved", "done"},
	"issue":     {kVerdict, kCommit, kAt, kRounds, kTokens, kDiff, kFiles},
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
	Text, Href, Class, Title, ISO, Tail string
}

// issueJudge builds an issue's strip from frontmatter only; a missing field
// adds no row.
func issueJudge(fm map[string]any) []prop {
	var out []prop
	if v := fmString(fm[kVerdict]); v != "" {
		class, glyph := "status status-done", "✓"
		if v != "pass" {
			class, glyph = "status status-escalated", "▲"
		}
		ps := []part{{Text: glyph + " " + v, Class: class}}
		if c := fmString(fm[kCommit]); c != "" {
			ps = append(ps, part{Text: "at"}, part{Text: c[:min(len(c), 7)], Class: "sha", Tail: ","})
		}
		if at := fmString(fm[kAt]); at != "" {
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
	if n := count(fm[kRounds]); n != "" {
		out = append(out, prop{Name: "Rounds", Values: []link{{Text: n, Plain: true}}})
	}
	if t := tokens(fm[kTokens]); t != "" {
		exact, _ := num(fm[kTokens])
		out = append(out, prop{Name: "Tokens", Rich: []part{{Text: t, Title: groupDigits(int64(exact))}}})
	}
	if d := count(fm[kDiff]); d != "" {
		change := d + " lines"
		if f := count(fm[kFiles]); f != "" {
			change += " in " + f + " files"
		}
		out = append(out, prop{Name: "Change", Values: []link{{Text: change, Plain: true}}})
	}
	return out
}

// judgedValue returns the part of a property that "All properties" still shows. An issue's
// external_links loses its pull URLs to the strip; ok is false when nothing remains.
func judgedValue(typ, name string, v any) (any, bool) {
	if typ != "issue" || name != "external_links" {
		return v, true
	}
	var out []any
	list, _ := v.([]any)
	for _, l := range list {
		if u, _ := l.(string); u != "" {
			if _, ok := prNumber(u); ok {
				continue
			}
		}
		out = append(out, l)
	}
	return out, isSet(out)
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
