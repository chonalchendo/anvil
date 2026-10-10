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
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.planned-new", map[string]any{"title": "Planned new", "status": "planned", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.planned-quiet", map[string]any{"title": "Planned quiet", "status": "planned", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0030-bare-slug", map[string]any{
		"title": "Bare slug", "status": "in-progress", "project": "anvil", "milestone": "[[milestone.planned-new]]", "updated": "2026-10-05",
		"verified_verdict": "pass", "cost_rounds": 2, "cost_tokens": 23300000, "owner": "a-worker",
		"external_links": []any{"https://github.com/o/r/pull/486/files"},
	}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0031-dropped", map[string]any{"title": "Dropped", "status": "abandoned", "project": "anvil", "milestone": "[[milestone.anvil.planned-new]]"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0032-shipped", map[string]any{"title": "Shipped", "status": "resolved", "project": "anvil", "milestone": "[[milestone.anvil.planned-new]]"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0040-homeless", map[string]any{"title": "Homeless", "status": "open", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0021-stranded", map[string]any{"title": "Stranded", "status": "open", "project": "anvil", "milestone": "[[milestone.anvil.shipped]]"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.shipped", map[string]any{"title": "Shipped one", "status": "done", "project": "anvil", "updated": "2026-09-01"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0020-old-issue", map[string]any{
		"title": "Old issue", "status": "resolved", "project": "anvil", "milestone": "[[milestone.anvil.old]]",
	}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0022-old-dropped", map[string]any{
		"title": "Old dropped", "status": "abandoned", "project": "anvil", "milestone": "[[milestone.anvil.old]]",
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
		`<div class="dashboard">`, `class="milestones"`, `class="work"`,
		`href="/artifact/milestone.anvil.live"`, `href="/artifact/milestone.anvil.old"`,
		`href="/artifact/system-design.anvil"`, `href="/artifact/product-design.anvil"`,
		`href="/diagram/anvil-two-loop"`, `href="/artifact/issue.anvil.0001-live-issue"`,
		`href="/artifact/decision.anvil.0009-late"`, `href="/type/issue?project=anvil"`,
		`<code translate="no">deadbee</code>`, `A deck line`, `loading="lazy"`,
		`href="/artifact/thread.anvil-design-docs.0001-ours"`, `href="/artifact/thread.anvil.0002-also-ours"`,
		`Still open:`, `(<time datetime="2026-10-07">7 Oct</time>)`, `(<time datetime="2026-10-08">8 Oct</time>)`, `1 of 1, <time datetime="2026-10-01">1 Oct</time>`, `<code translate="no">anvil-two-loop</code> The state lives in the vault.`,
		`href="/type/thread"`, `>3 threads</a>`, `9 Oct`, `not approved`, `not measured`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard lacks %q", want)
		}
	}
	for _, bad := range []string{"Foreign thread", "Mentat thread", ">2026-10-08<"} {
		if strings.Contains(body, bad) {
			t.Errorf("dashboard holds %q", bad)
		}
	}
	if strings.Contains(body, `class="inventory"`) || strings.Contains(body, `class="bar"`) {
		t.Error("the inventory column is back")
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

// Warrant: fails if live work under a planned milestone is hidden, a bare-slug issue lands in the wrong fold,
// the counts count abandoned issues, or an issue row drops its verdict, PR, rounds or tokens.
func TestProject_MilestoneFolds(t *testing.T) {
	body, _ := seedProject(t)
	for _, want := range []string{
		`<details class="ms live" open>`, `<details class="ms">`, `>No open milestone<`,
		`<a href="/artifact/milestone.anvil.planned-new" class="to-planned">Planned new</a>`,
		`1 of 2</span>`, `1 abandoned, not counted in the 2.`,
		`>0030</span>`, `pass`, `<a href="https://github.com/o/r/pull/486/files" translate="no">#486</a>`, `>2</td>`, `>23.3M</td>`, `a-worker`, `5 Oct`,
		`href="/artifact/issue.anvil.0040-homeless"`, `href="/artifact/milestone.anvil.planned-quiet"`, `No open issue.`,
		`3 milestones are not done: 1 in progress and 2 planned.`, `11 open issues sit under the 3 milestones, and 2 under none or a done milestone.`,
		`href="/artifact/issue.anvil.0021-stranded"`, `<time datetime="2026-10-05">5 Oct</time>`, `translate="no">a-worker`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("folds lack %q", want)
		}
	}
	if strings.Contains(body, `class="empty"`) {
		t.Error("the empty-state inset shows while work is in progress")
	}
	order := []string{"Planned new", "Live one", "Planned quiet", ">No open milestone<"}
	last := -1
	for _, title := range order {
		i := strings.Index(body, title)
		if i < last {
			t.Errorf("%q is out of order", title)
		}
		last = i
	}
}

// Warrant: fails if the empty-state inset shows while work is in progress, or says nothing when nothing is.
func TestProject_EmptyState(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.next", map[string]any{"title": "Next one", "status": "planned", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "anvil.0050-queued", map[string]any{"title": "Queued", "status": "open", "project": "anvil", "milestone": "[[milestone.anvil.next]]"}, "x\n")
	// The seeded in-progress stack issue sits under no project milestone, so close it out of the picture.
	writeArtifact(t, v, core.TypeIssue, stackIssue, map[string]any{"title": "Thing", "status": "resolved", "project": "anvil", "milestone": "[[milestone.anvil.next]]"}, "x\n")
	body, _ := projectBody(t, h)
	if strings.Contains(body, "without live work") {
		t.Error("the lead names a bare tier with no in-progress milestone")
	}
	if !strings.Contains(body, `Nothing is in progress. 1 issue is open: 1 under the 1 planned milestone;`) || !strings.Contains(body, `Next one</a> holds the most`) {
		t.Errorf("empty state lacks the open-issue sentence:\n%s", body)
	}
}

func TestTokens(t *testing.T) {
	for in, want := range map[any]string{23300000: "23.3M", 12400: "12.4k", 840: "840", "x": "", nil: ""} {
		if got := tokens(in); got != want {
			t.Errorf("tokens(%v) = %q, want %q", in, got, want)
		}
	}
}

// Warrant: fails if the nothing-planned inset drops the product design's #milestones link.
func TestProject_InsetLinksProductDesign(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeProductDesign, "anvil", map[string]any{"title": "Anvil product", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, stackIssue, map[string]any{"title": "Thing", "status": "resolved", "project": "anvil"}, "x\n")
	body, _ := projectBody(t, h)
	if !strings.Contains(body, `No milestone is in flight or planned. The product design lists the next candidates under <a href="/artifact/product-design.anvil#milestones">Milestones</a>.`) {
		t.Errorf("inset lacks the product design link:\n%s", body)
	}
}

// Warrant: fails if the lead omits "No milestone is in progress without live work." when every in-progress milestone holds live work.
func TestProject_NoBareInProgressMilestone(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.live", map[string]any{"title": "Live one", "status": "in-progress", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, stackIssue, map[string]any{"title": "Thing", "status": "in-progress", "project": "anvil", "milestone": "[[milestone.anvil.live]]"}, "x\n")
	body, _ := projectBody(t, h)
	if !strings.Contains(body, "No milestone is in progress without live work.") {
		t.Errorf("lead lacks the no-bare-tier sentence:\n%s", body)
	}
}
