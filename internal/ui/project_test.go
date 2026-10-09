package ui

import (
	"net/http"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func seedProject(t *testing.T) (string, int) {
	t.Helper()
	h, v := seed(t)
	writeArtifact(t, v, core.TypeProductDesign, "anvil", map[string]any{"title": "Anvil product", "project": "anvil", "description": "A deck line"}, "x\n")
	writeArtifact(t, v, core.TypeSystemDesign, "anvil", map[string]any{"title": "Anvil system", "project": "anvil", "diagrams": []any{"anvil-two-loop"}}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.live", map[string]any{
		"title": "Live one", "status": "in-progress", "project": "anvil", "approved": "2026-10-09",
	}, "## Status\n\nMeasured: 2026-10-09, at `abc1234`. No predicate passes.\n\n| # | AC | Met | Measured |\n|---|---|---|---|\n| 1 | `/project/anvil` dashboard | not met | 404 |\n| 2 | sidebar links | met | ok |\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.old", map[string]any{
		"title": "Old one", "status": "done", "project": "anvil", "updated": "2026-10-01",
	}, "## Status\n\nMeasured: 2026-10-01, at `deadbee`. Every predicate passes.\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0001-live-issue", map[string]any{
		"title": "Live issue", "status": "open", "project": "anvil", "milestone": "[[milestone.anvil.live]]",
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
		`<code>deadbee</code>`, `class="judge"`, `A deck line`, `status-not-met`, `/project/anvil dashboard`,
		`0 of 1 issues resolved`, `class="bar"`, `style="--n:1"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard lacks %q", want)
		}
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
	if len(got) != 1 || got[0].Text != "a | b" || got[0].Met != "not met" || got[0].Class != "status-not-met" || got[0].Measured != "404" {
		t.Errorf("acceptance = %+v", got)
	}
}

func TestMeasuredSHA(t *testing.T) {
	if got := measuredSHA("## Status\n\nMeasured: 2026-10-09, at `f280530`. ok\n"); got != "f280530" {
		t.Errorf("measuredSHA = %q", got)
	}
}
