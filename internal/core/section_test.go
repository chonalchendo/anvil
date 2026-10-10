package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestScanSection(t *testing.T) {
	const body = "intro\n" +
		"## A\n" +
		"a1\n" +
		"### Sub\n" +
		"sub1\n" +
		"#### Deep\n" +
		"deep1\n" +
		"### Sub2\n" +
		"sub2\n" +
		"## B\n" +
		"b1\n" +
		"~~~\n" +
		"## not a heading\n" +
		"~~~\n" +
		"````\n" +
		"```\n" +
		"## also not\n" +
		"```\n" +
		"````\n" +
		"b2\n" +
		"# Top\n" +
		"top1\n" +
		"## C\n" +
		"```go\n" +
		"## inside\n" +
		"```\n" +
		"c1\n"
	tests := []struct {
		name, want, text string
		found            bool
	}{
		{"bare name reads as H2", "A", "## A\na1\n### Sub\nsub1\n#### Deep\ndeep1\n### Sub2\nsub2", true},
		{"H3 stops at sibling H3", "### Sub", "### Sub\nsub1\n#### Deep\ndeep1", true},
		{"H3 nests H4", "### Sub2", "### Sub2\nsub2", true},
		{"tilde and 4-backtick fences hide headings; H1 stops", "B", "## B\nb1\n~~~\n## not a heading\n~~~\n````\n```\n## also not\n```\n````\nb2", true},
		{"info-string line inside fence stays inside", "C", "## C\n```go\n## inside\n```\nc1", true},
		{"heading inside fence is not found", "not a heading", "", false},
		{"missing", "Nope", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, h2s, found := ScanSection(body, tt.want)
			if found != tt.found || text != tt.text {
				t.Errorf("ScanSection(%q) = %q, %v; want %q, %v", tt.want, text, found, tt.text, tt.found)
			}
		})
	}
}

func TestScanSectionH2Order(t *testing.T) {
	_, h2s, _ := ScanSection("## Z\n## A\n### x\n## M\n", "none")
	if diff := cmp.Diff([]string{"## Z", "## A", "## M"}, h2s); diff != "" {
		t.Errorf("h2s (-want +got):\n%s", diff)
	}
}

func TestFenceOpenBacktickInfoString(t *testing.T) {
	if c, n := fenceOpen("``` a`b"); c != 0 || n != 0 {
		t.Errorf("backtick in backtick-fence info string opened a fence: %q %d", c, n)
	}
	if c, n := fenceOpen("~~~ a`b"); c != '~' || n != 3 {
		t.Errorf("tilde fence with backtick info = %q %d; want ~ 3", c, n)
	}
	if c, _ := fenceOpen("```go"); c != '`' {
		t.Errorf("plain info string must open a fence")
	}
}
