package ui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func seedProject(t *testing.T) (string, int) {
	t.Helper()
	h, v := seed(t)
	writeArtifact(t, v, core.TypeProductDesign, "anvil", map[string]any{"title": "Anvil product", "project": "anvil", "description": "A deck line"}, "x\n")
	writeArtifact(t, v, core.TypeSystemDesign, "anvil", map[string]any{"title": "Anvil system", "project": "anvil", "diagrams": []any{"anvil-two-loop"}}, "Work runs in two loops. The state lives in the vault.\n\n*Diagram `anvil-two-loop` renders on this page's canvas.*\n")
	writeArtifact(t, v, core.TypeThread, "anvil-design-docs.0001-ours", map[string]any{"title": "Our thread", "status": "open", "updated": "2026-10-07"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "anvil.0002-also-ours", map[string]any{"title": "Also ours", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "anvilish.0001-foreign", map[string]any{"title": "Foreign thread", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "mentat.0001-other", map[string]any{"title": "Mentat thread", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.live", map[string]any{
		"title": "Live one", "status": "in-progress", "project": "anvil", "approved": "2026-10-09",
	}, "## Status\n\nMeasured: 2026-10-09, at `abc1234`. No predicate passes.\n\n| # | AC | Met | Measured |\n|---|---|---|---|\n| 1 | `/project/anvil` dashboard | not met | 404 |\n| 2 | sidebar links | met | ok |\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.old", map[string]any{
		"title": "Old one", "status": "done", "project": "anvil", "updated": "2026-10-01",
	}, "## Status\n\nMeasured: 2026-10-01, at `deadbee`. Every predicate passes.\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0001-live-issue", map[string]any{
		"title": "Live issue", "status": "open", "project": "anvil", "milestone": "[[milestone.anvil.live]]",
	}, "x\n")
	for i := 2; i <= 11; i++ {
		writeArtifact(t, v, core.TypeIssue, fmt.Sprintf("anvil.%04d-more-%d", i, i), map[string]any{
			"title": "More", "status": "open", "project": "anvil", "milestone": "[[milestone.anvil.live]]",
		}, "x\n")
	}
	writeArtifact(t, v, core.TypeIssue, "anvil.0020-old-issue", map[string]any{
		"title": "Old issue", "status": "resolved", "project": "anvil", "milestone": "[[milestone.anvil.old]]",
	}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "anvil.0009-late", map[string]any{"title": "Late decision", "status": "accepted", "project": "anvil", "updated": "2026-10-08"}, "x\n")
	return projectBody(t, h)
}

func projectBody(t *testing.T, h http.Handler) (string, int) {
	t.Helper()
	code, body := do(h, "GET", "/project/anvil")
	return body, code
}

// Warrant: fails if the dashboard drops a pane, a link the human follows, or a milestone's acceptance table.
func TestProject_Dashboard(t *testing.T) {
	body, code := seedProject(t)
	if code != 200 {
		t.Fatalf("GET /project/anvil = %d", code)
	}
	for _, want := range []string{
		`<div class="dashboard">`, `class="inventory"`, `class="work"`,
		`href="/artifact/milestone.anvil.live"`, `href="/artifact/milestone.anvil.old"`,
		`href="/artifact/system-design.anvil"`, `href="/artifact/product-design.anvil"`,
		`href="/diagram/anvil-two-loop"`, `href="/artifact/issue.anvil.0001-live-issue"`,
		`href="/artifact/decision.anvil.0009-late"`, `href="/type/issue?project=anvil"`,
		`<code>deadbee</code>`, `class="judge"`, `A deck line`, `status-not-met`,
		`0 of 11 issues resolved`, `class="bar"`, `style="--n:1"`,
		`<i class="status-open status-planned"`, `<code>/project/anvil</code> dashboard`,
		`href="/type/issue?project=anvil"`, `href="/type/issue?to=milestone.anvil.live&amp;status=open"`, `3 more`,
		`href="/artifact/thread.anvil-design-docs.0001-ours"`, `href="/artifact/thread.anvil.0002-also-ours"`,
		`Still open:`, `(7 Oct)`, `(8 Oct)`, `1 of 1, 1 Oct`, `<code>anvil-two-loop</code> The state lives in the vault.`,
		`href="/type/thread"`, `>2 open</span>`, `loading="lazy"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard lacks %q", want)
		}
	}
	for _, bad := range []string{"Foreign thread", "Mentat thread", "barClass", "2026-10-08"} {
		if strings.Contains(body, bad) {
			t.Errorf("dashboard holds %q", bad)
		}
	}
	if strings.Count(body, `class="inv"`) != 1 || strings.Count(body, ">designs</a>") != 1 {
		t.Error("designs are not one inventory row")
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Error("template escaper rejected a value")
	}
}

func TestProject_UnknownIs404(t *testing.T) {
	h, _ := seed(t)
	if code, _ := do(h, "GET", "/project/nope"); code != 404 {
		t.Errorf("GET /project/nope = %d, want 404", code)
	}
	if code, _ := do(h, "POST", "/project/anvil"); code != 405 {
		t.Errorf("POST /project/anvil = %d, want 405", code)
	}
}

func TestAcceptance(t *testing.T) {
	got := acceptance("## Other\n\n| 9 | x | met | y |\n\n## Status\n\n| # | AC | Met | Measured |\n|---|---|---|---|\n| 1 | `a | b` | not met | 404 |\n")
	if len(got) != 1 || got[0].Text != "<code>a | b</code>" || got[0].Met != "not met" || got[0].Class != "status-not-met" || got[0].Measured != "404" {
		t.Errorf("acceptance = %+v", got)
	}
}

func TestMeasuredSHA(t *testing.T) {
	if got := measuredSHA("## Status\n\nMeasured: 2026-10-09, at `f280530`. ok\n"); got != "f280530" {
		t.Errorf("measuredSHA = %q", got)
	}
}

func TestShortDate(t *testing.T) {
	for in, want := range map[string]string{"2026-10-09": "9 Oct", "2026-10-09T10:00:00Z": "9 Oct", "": "", "soon": "soon"} {
		if got := shortDate(in); got != want {
			t.Errorf("shortDate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiagramNote(t *testing.T) {
	body := "## A\n\nFirst. The **loop** runs.\n\n*Diagram `x` renders here.*\n\n## B\n\n*Diagram `y` renders here.*\n"
	if got := diagramNote(body, "x"); got != "The loop runs." {
		t.Errorf("diagramNote x = %q", got)
	}
	if got := diagramNote(body, "y"); got != "" {
		t.Errorf("diagramNote y = %q, want empty under a heading", got)
	}
}

// Warrant: fails if "N more" filters by status when the hidden issues differ, or drops the filter when they agree.
func TestProject_MoreHrefStatusFilter(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.mixed", map[string]any{"title": "Mixed", "status": "in-progress", "project": "anvil"}, "x\n")
	for i := 1; i <= treeCap+2; i++ {
		status := "in-progress"
		if i == treeCap+2 {
			status = "open"
		}
		writeArtifact(t, v, core.TypeIssue, fmt.Sprintf("anvil.%04d-mixed-%d", i, i), map[string]any{
			"title": "Mixed", "status": status, "project": "anvil", "milestone": "[[milestone.anvil.mixed]]",
		}, "x\n")
	}
	body, _ := projectBody(t, h)
	if !strings.Contains(body, `href="/type/issue?to=milestone.anvil.mixed"`) || strings.Contains(body, "milestone.anvil.mixed&amp;status") {
		t.Error("mixed hidden statuses must not filter the More link")
	}
}
