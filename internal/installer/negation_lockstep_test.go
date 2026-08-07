package installer

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// Two executors judge a verification block: the shipped run-verification.sh
// (awk) and anvil's create-time gate (core.NonGatingNegation). If they drift, a
// block `anvil create issue` accepts can still false-pass in the fleet runner —
// the exact hole mentat.0291 closed. So the rule gets ONE case corpus, driven
// through both here rather than two hand-matched regexes maintained in parallel.
//
// Each case is the Direct block of a synthetic issue; `want` is the offending
// line both executors must name (trimmed), or "" for a block they must accept.
// The exempt-position set is bash-verified: `;`, `&&`, `||`, `do`, `then` and
// `{` all let a failing `! cmd` survive, while `( ! cmd )` aborts — so the
// subshell case must NOT be refused.

const negationPrefix = "non-gating negation: "

var negationCorpus = []struct {
	name  string
	block string
	want  string
}{
	{"line-led negation is vacuous", "! true\necho SURVIVED", "! true"},
	{"negation after a semicolon is vacuous", "echo a; ! true\necho SURVIVED", "echo a; ! true"},
	{"negation after && is vacuous", "true && ! true\necho SURVIVED", "true && ! true"},
	{"negation after || is vacuous", "false || ! true\necho SURVIVED", "false || ! true"},
	{"one-line loop body negation is vacuous", "for f in a b; do ! test -f $f; done\necho SURVIVED", "for f in a b; do ! test -f $f; done"},
	{"loop body negation gates only on the final iteration", "for f in a b; do\n  ! test -f $f\ndone\necho SURVIVED", "! test -f $f"},
	{"then-branch negation is vacuous", "if true; then ! true; fi\necho SURVIVED", "if true; then ! true; fi"},
	{"brace-group negation is vacuous", "{ ! true; }\necho SURVIVED", "{ ! true; }"},
	{"the detector is textual, so a quoted bang in command position is refused too", "echo 'x; ! y'\necho ok", "echo 'x; ! y'"},

	{"last-line negation gates", "echo hi\n! false", ""},
	{"trailing comment does not displace the last line", "! false\n# done", ""},
	{"subshell negation gates, so it is not refused", "( ! false )\necho ok", ""},
	{"if-guarded negation is the recommended rewrite", "if false; then exit 1; fi\necho ok", ""},
	{"negation in an if condition gates", "if ! false; then echo ok; fi\necho fine", ""},
	{"!= is not a negation", "[ \"a\" != \"b\" ]\necho ok", ""},
	{"test's ! primary is not a command negation", "[ ! -f /nonexistent ]\necho ok", ""},
	{"find's ! primary is not a command negation", "find . -maxdepth 0 ! -name zzz >/dev/null\necho ok", ""},
}

func TestNonGatingNegation_ExecutorsAgree(t *testing.T) {
	for _, c := range negationCorpus {
		t.Run(c.name, func(t *testing.T) {
			if got := core.NonGatingNegation(c.block); got != c.want {
				t.Errorf("core.NonGatingNegation(%q) = %q, want %q", c.block, got, c.want)
			}

			v, stderr, _ := runVerification(t, issueDoc(c.block, "true"))
			got := ""
			for _, f := range v.Failed {
				if f.Check == "Direct#1" && strings.HasPrefix(f.Preview, negationPrefix) {
					got = strings.TrimPrefix(f.Preview, negationPrefix)
				}
			}
			if got != c.want {
				t.Errorf("run-verification.sh refused %q, want %q\nstderr:\n%s", got, c.want, stderr)
			}
		})
	}
}
