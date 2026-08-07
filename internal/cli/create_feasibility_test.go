package cli

import "testing"

// If nonGatingNegation stopped flagging a mid-block `! cmd`, the feasibility
// gate would again accept a predicate that set -e silently skips (mentat.0291).
func TestNonGatingNegation(t *testing.T) {
	cases := []struct {
		name  string
		block string
		want  string
	}{
		{"mid-block negation is vacuous", "! true\necho SURVIVED", "! true"},
		{"last-line negation gates", "echo hi\n! true", ""},
		{"trailing comment does not displace the last line", "! true\n# done", ""},
		{"loop body negation is vacuous", "for f in a b; do\n  ! test -f $f\ndone", "! test -f $f"},
		{"if-guarded negation is fine", "if true; then exit 1; fi\necho ok", ""},
		{"history-style bang is not a negation", "!! foo\necho ok", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nonGatingNegation(c.block); got != c.want {
				t.Fatalf("nonGatingNegation(%q) = %q, want %q", c.block, got, c.want)
			}
		})
	}
}
