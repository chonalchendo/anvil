package facets_test

import (
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/facets"
)

func TestDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"dbt", "dtb", 2}, // adjacent transposition costs two single-char edits
		{"pyhton", "python", 2},
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "", 3},
	}
	for _, c := range cases {
		if got := facets.Distance(c.a, c.b); got != c.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
