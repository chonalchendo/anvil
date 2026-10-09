package ui

import (
	"fmt"
	"strings"
)

// judgeKeys are the frontmatter fields a reader judges a node by, per type.
// Only these types get a strip; the keys leave "All properties".
var judgeKeys = map[string][]string{
	"learning":  {"confidence", "diataxis"},
	"decision":  {"date", "supersedes", "superseded_by"},
	"milestone": {"approved", "done"},
}

// judge builds the judge strip: set frontmatter fields in judgeKeys order,
// then a milestone's last Measured: line.
func (s *server) judge(typ string, fm map[string]any, body string) []prop {
	keys, ok := judgeKeys[typ]
	if !ok {
		return nil
	}
	var out []prop
	for _, k := range keys {
		if isSet(fm[k]) {
			out = append(out, s.prop(k, fm[k]))
		}
	}
	if typ == "milestone" {
		if m := lastMeasured(body); m != "" {
			out = append(out, prop{Name: "measured", Values: []link{{Text: m, Plain: true}}})
		}
	}
	return out
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

// lastMeasured returns the last "Measured:" line, label dropped, inside the Status section.
func lastMeasured(body string) string {
	var got string
	in := false
	for line := range strings.SplitSeq(body, "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			in = strings.TrimSpace(h) == "Status"
			continue
		}
		if rest, ok := strings.CutPrefix(line, "Measured:"); in && ok {
			got = strings.TrimSpace(strings.ReplaceAll(rest, "`", ""))
		}
	}
	return got
}
