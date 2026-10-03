package core

import "testing"

func TestIsPostLand_FirstNonBlankLine_MatchesMarker(t *testing.T) {
	cases := map[string]bool{
		"# anvil:post-land\nfalse\n":      true,
		"\n  # anvil:post-land  \ntrue\n": true,
		"false\n# anvil:post-land\n":      false,
		"# note\nfalse\n":                 false,
		"":                                false,
	}
	for block, want := range cases {
		if got := IsPostLand(block); got != want {
			t.Errorf("IsPostLand(%q) = %v, want %v", block, got, want)
		}
	}
}
