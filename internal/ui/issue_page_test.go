package ui

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeJudged(t *testing.T, v *core.Vault, id string, extra map[string]any) {
	t.Helper()
	fm := map[string]any{"title": "Judged", "status": "resolved", "project": "anvil", "milestone": "[[milestone.anvil.m2]]"}
	for k, val := range extra {
		fm[k] = val
	}
	writeArtifact(t, v, core.TypeIssue, id, fm, "b\n")
}

// Warrant: each issue judge field shows from frontmatter, in mockup order, with the
// short token form carrying the exact count; if a field's branch is dropped its row vanishes.
func TestIssueJudge_ShowsEveryFieldInOrder(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.m2", map[string]any{"title": "M2", "project": "anvil"}, "x\n")
	writeJudged(t, v, "anvil.0500.full", map[string]any{
		"verified_verdict": "pass", "verified_commit": "66806c92e632174e", "verified_at": "2026-10-09T23:16:52Z",
		"external_links": []any{"https://example.com/x", "https://github.com/o/r/pull/515"},
		"cost_rounds":    3, "cost_tokens": 23313576, "cost_diff": 861, "cost_files": 25, "tags": []any{"x"},
	})
	_, body := do(h, "GET", "/artifact/issue.anvil.0500.full")
	strip := judgeStrip(t, body)
	var last int
	for _, want := range []string{
		">Verdict<", `class="status status-done">✓ pass<`, `class="sha">66806c9<`, `<time datetime="2026-10-09T23:16:52Z">9 Oct</time>`,
		">PR<", `href="https://github.com/o/r/pull/515" translate="no">#515<`,
		">Rounds<", ">3<", ">Tokens<", `title="23,313,576">23.3M<`, ">Change<", ">861 lines in 25 files<",
	} {
		at := strings.Index(strip, want)
		if at < last {
			t.Errorf("strip lacks %q after offset %d: %s", want, last, strip)
		}
		last = max(last, at)
	}
	if strings.Contains(strip, "example.com") {
		t.Error("a non-pull link showed as a PR")
	}
	_, props, _ := strings.Cut(body, `<details class="props"`)
	for _, k := range []string{"verified_verdict", "external_links", "cost_tokens", "cost_files"} {
		if strings.Contains(props, "<dt>"+k+"</dt>") {
			t.Errorf("%s still in All properties", k)
		}
	}
	if !strings.Contains(props, "<dt>tags</dt>") {
		t.Error("non-judge key left props")
	}
}

// Warrant: a failing verdict must not wear the pass colour.
func TestIssueJudge_FailVerdictAndMissingFields(t *testing.T) {
	h, v := seed(t)
	writeJudged(t, v, "anvil.0501.fail", map[string]any{"verified_verdict": "fail"})
	_, body := do(h, "GET", "/artifact/issue.anvil.0501.fail")
	strip := judgeStrip(t, body)
	if !strings.Contains(strip, `status-escalated">▲ fail<`) || strings.Contains(strip, "status-done") {
		t.Errorf("fail verdict strip: %s", strip)
	}
	for _, no := range []string{">PR<", ">Rounds<", ">Tokens<", ">Change<", " at "} {
		if strings.Contains(strip, no) {
			t.Errorf("absent field rendered %q", no)
		}
	}
}

// Warrant: an issue's crumb is project then its milestone's title, with a bare milestone slug
// resolved, and no type word or repeat of the current node.
func TestIssueCrumbs_ProjectThenMilestoneTitle(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.m2", map[string]any{
		"title": "Second milestone", "project": "anvil", "product_design": "[[product-design.anvil]]",
	}, "x\n")
	writeJudged(t, v, "anvil.0502.bare", map[string]any{"milestone": "[[milestone.m2]]"})
	_, body := do(h, "GET", "/artifact/issue.anvil.0502.bare")
	crumbs := crumbsOf(t, body)
	proj, ms := strings.Index(crumbs, ">anvil</a>"), strings.Index(crumbs, ">Second milestone</a>")
	if proj < 0 || ms < proj {
		t.Errorf("crumbs not project then milestone title: %s", crumbs)
	}
	for _, no := range []string{"Anvil product", ">issue<", ">milestone<", "Judged"} {
		if strings.Contains(crumbs, no) {
			t.Errorf("crumbs hold %q: %s", no, crumbs)
		}
	}
}

// Warrant: a milestone page lists the issues that name it, bare slugs included, in the
// dashboard's issue table, and not the issues of another milestone.
func TestMilestonePage_ListsItsIssues(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.m2", map[string]any{"title": "M2", "project": "anvil"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.m3", map[string]any{"title": "M3", "project": "anvil"}, "x\n")
	writeJudged(t, v, "anvil.0503.full", map[string]any{"updated": "2026-10-01"})
	writeJudged(t, v, "anvil.0504.bare", map[string]any{"milestone": "[[milestone.m2]]", "updated": "2026-10-05"})
	writeJudged(t, v, "anvil.0505.other", map[string]any{"milestone": "[[milestone.anvil.m3]]"})
	_, body := do(h, "GET", "/artifact/milestone.milestone.anvil.m2")
	_, table, ok := strings.Cut(body, `<table class="iss"`)
	if !ok {
		t.Fatal("issue table missing")
	}
	table, _, _ = strings.Cut(table, "</table>")
	newer, older := strings.Index(table, "issue.anvil.0504.bare"), strings.Index(table, "issue.anvil.0503.full")
	if newer < 0 || older < newer {
		t.Errorf("rows missing or not newest first: %s", table)
	}
	if strings.Contains(table, "0505") {
		t.Error("another milestone's issue listed")
	}
}

// Warrant: a milestone with no issue shows an empty state, not an empty table.
func TestMilestonePage_EmptyState(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.m4", map[string]any{"title": "M4", "project": "anvil"}, "x\n")
	_, body := do(h, "GET", "/artifact/milestone.milestone.anvil.m4")
	if strings.Contains(body, `<table class="iss"`) || !strings.Contains(body, "No issues yet.") {
		t.Error("empty milestone should show the empty state only")
	}
}
