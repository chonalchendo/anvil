package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// Warrant: fails if the band drops a column, lists a non-flight status, omits the capped "N more" link, or shows drafts oldest-first.
func TestNowBand(t *testing.T) {
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.live", map[string]any{"title": "Live", "status": "in-progress", "project": "p"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.next", map[string]any{"title": "Next", "status": "planned", "project": "p"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.old", map[string]any{"title": "Old", "status": "done", "project": "p"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "p.0001-open", map[string]any{"title": "Open thread", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "p.0002-shut", map[string]any{"title": "Shut thread", "status": "closed"}, "x\n")
	for i := 0; i < treeCap+2; i++ {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("d%02d", i), map[string]any{"title": fmt.Sprintf("Draft %02d", i), "status": "draft", "updated": fmt.Sprintf("2026-10-%02d", 10+i)}, "x\n")
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
		`href="/artifact/thread.p.0001-open"`, `href="/artifact/learning.d09"`,
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
	drafts := band[strings.Index(band, "Draft learnings"):strings.Index(band, "Recently updated")]
	if !strings.Contains(drafts, "learning.d09") || strings.Contains(drafts, "learning.d00") {
		t.Errorf("draft column is not newest-first (newest d09 must show, oldest d00 must not):\n%s", drafts)
	}
	if start > strings.Index(body, "<h1>Design spine</h1>") {
		t.Error("band sits below the tree")
	}
}

func nowBandBody(t *testing.T, v *core.Vault) string {
	t.Helper()
	code, body := homeBody(t, v)
	if code != 200 {
		t.Fatalf("GET / = %d", code)
	}
	start := strings.Index(body, `<section class="now"`)
	if start < 0 {
		t.Fatal("home lacks the Now band")
	}
	return body[start : start+strings.Index(body[start:], "</section>")]
}

// Warrant: fails if a column exactly at the cap grows a "more" link.
func TestNowBand_ExactlyCapHasNoMore(t *testing.T) {
	v := &core.Vault{Root: t.TempDir()}
	for i := 0; i < treeCap; i++ {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("d%02d", i), map[string]any{"title": fmt.Sprintf("Draft %02d", i), "status": "draft"}, "x\n")
	}
	if band := nowBandBody(t, v); strings.Contains(band, `class="more"`) {
		t.Errorf("column at the cap has a more link:\n%s", band)
	}
}

// Warrant: fails if the milestone "more" link filters to one status while hidden rows span two, losing the planned row.
func TestNowBand_MilestoneMoreSpansStatuses(t *testing.T) {
	v := &core.Vault{Root: t.TempDir()}
	for i := 0; i < treeCap+1; i++ {
		writeArtifact(t, v, core.TypeMilestone, fmt.Sprintf("milestone.p.live%02d", i), map[string]any{"title": "Live", "status": "in-progress", "project": "p", "updated": fmt.Sprintf("2026-10-%02d", i+1)}, "x\n")
	}
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.next", map[string]any{"title": "Next", "status": "planned", "project": "p", "updated": "2026-01-01"}, "x\n")
	band := nowBandBody(t, v)
	if !strings.Contains(band, `href="/type/milestone">2 more`) {
		t.Errorf("hidden rows span two statuses but more link is not unfiltered:\n%s", band)
	}
}

// Warrant: fails if the cap slice shows more or fewer than treeCap rows when one row is hidden.
func TestNowBand_CapPlusOneShowsOneMore(t *testing.T) {
	v := &core.Vault{Root: t.TempDir()}
	for i := 0; i < treeCap+1; i++ {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("d%02d", i), map[string]any{"title": fmt.Sprintf("Draft %02d", i), "status": "draft", "updated": fmt.Sprintf("2026-10-%02d", 10+i)}, "x\n")
	}
	band := nowBandBody(t, v)
	drafts := band[strings.Index(band, "Draft learnings"):strings.Index(band, "Recently updated")]
	if got := strings.Count(drafts, `href="/artifact/learning.`); got != treeCap {
		t.Errorf("draft column shows %d rows, want %d", got, treeCap)
	}
	if !strings.Contains(drafts, `>1 more</a>`) {
		t.Errorf("draft column lacks the 1 more link:\n%s", drafts)
	}
}
