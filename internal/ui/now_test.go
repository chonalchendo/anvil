package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// Warrant: fails if the band drops a column, lists a non-flight status, or omits the capped "N more" link.
func TestNowBand(t *testing.T) {
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.live", map[string]any{"title": "Live", "status": "in-progress", "project": "p"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.next", map[string]any{"title": "Next", "status": "planned", "project": "p"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.old", map[string]any{"title": "Old", "status": "done", "project": "p"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "p.0001-open", map[string]any{"title": "Open thread", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "p.0002-shut", map[string]any{"title": "Shut thread", "status": "closed"}, "x\n")
	for i := 0; i < nowCap+2; i++ {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("d%02d", i), map[string]any{"title": fmt.Sprintf("Draft %02d", i), "status": "draft"}, "x\n")
	}
	code, body := homeBody(t, v)
	if code != 200 {
		t.Fatalf("GET / = %d", code)
	}
	start := strings.Index(body, `<section class="now"`)
	if start < 0 {
		t.Fatal("home lacks the Now band")
	}
	band := body[start : start+strings.Index(body[start:], "</section>")]
	for _, want := range []string{
		`href="/artifact/milestone.p.live"`, `href="/artifact/milestone.p.next"`,
		`href="/artifact/thread.p.0001-open"`, `href="/artifact/learning.d00"`,
		`2 more`, `href="/type/learning?status=draft"`, `Recently updated`,
	} {
		if !strings.Contains(band, want) {
			t.Errorf("band lacks %q", want)
		}
	}
	for _, bad := range []string{"milestone.p.old", "thread.p.0002-shut"} {
		if strings.Contains(band, bad) {
			t.Errorf("band lists %q", bad)
		}
	}
	if start > strings.Index(body, "<h1>Design spine</h1>") {
		t.Error("band sits below the tree")
	}
}
