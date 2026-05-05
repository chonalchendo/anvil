package output_test

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/output"
)

func TestTruncationHint_Truncated(t *testing.T) {
	got := output.TruncationHint("most recent", 10, 312, []string{"--since/--until", "--status", "--type", "--tag", "--project"})
	want := "showing 10 of 312 most recent; narrow with --since/--until, --status, --type, --tag, --project, or raise --limit"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTruncationHint_NotTruncated(t *testing.T) {
	got := output.TruncationHint("most recent", 5, 5, []string{"--status"})
	if got != "" {
		t.Errorf("expected empty hint, got %q", got)
	}
}

func TestBodyClipHint(t *testing.T) {
	got := output.BodyClipHint(500, 1240, "03-issues/issue-42-foo.md")
	if !strings.Contains(got, "500 of 1240") || !strings.Contains(got, "03-issues/issue-42-foo.md") {
		t.Errorf("hint missing expected content: %q", got)
	}
}
