package installer

import (
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// Two executors decide whether a block is post-land: run-verification.sh (which
// defers a red one) and core.IsPostLand (which --land-pr uses to run it after
// merge). If they disagree, a block is deferred yet never run, and a red check
// false-passes for good. One case table drives both.
func TestPostLand_ExecutorsAgree(t *testing.T) {
	cases := []struct {
		name  string
		block string
		// marked: script reports the red Indirect block under deferred, and IsPostLand is true.
		marked bool
	}{
		{"exact marker", "# anvil:post-land\nfalse", true},
		{"leading blank lines and edge whitespace", "\n  # anvil:post-land  \nfalse", true},
		{"no space after hash", "#anvil:post-land\nfalse", false},
		{"space after colon", "# anvil: post-land\nfalse", false},
		{"double space after hash", "#  anvil:post-land\nfalse", false},
		{"marker not on the first line", "false\n# anvil:post-land", false},
		{"NBSP is not whitespace to either side", "\u00a0# anvil:post-land\nfalse", false},
		{"unmarked", "false", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := core.IsPostLand(c.block); got != c.marked {
				t.Errorf("core.IsPostLand(%q) = %v, want %v", c.block, got, c.marked)
			}
			v, stderr, _ := runVerification(t, issueDoc("true", c.block))
			if got := len(v.Deferred) == 1; got != c.marked {
				t.Errorf("run-verification.sh deferred = %v, want %v\nstderr:\n%s", got, c.marked, stderr)
			}
			wantVerdict := "fail"
			if c.marked {
				wantVerdict = "pass"
			}
			if v.Verdict != wantVerdict {
				t.Errorf("verdict = %q, want %q", v.Verdict, wantVerdict)
			}
		})
	}

	t.Run("red marked block in Direct is never deferred", func(t *testing.T) {
		v, _, _ := runVerification(t, issueDoc("# anvil:post-land\nfalse", "true"))
		if v.Verdict != "fail" || len(v.Deferred) != 0 {
			t.Errorf("verdict=%q deferred=%d, want fail with none deferred", v.Verdict, len(v.Deferred))
		}
	})
	t.Run("passing marked block is neither failed nor deferred", func(t *testing.T) {
		v, _, _ := runVerification(t, issueDoc("true", "# anvil:post-land\ntrue"))
		if v.Verdict != "pass" || len(v.Deferred) != 0 || len(v.Failed) != 0 {
			t.Errorf("verdict=%q deferred=%d failed=%d, want clean pass", v.Verdict, len(v.Deferred), len(v.Failed))
		}
	})
}
