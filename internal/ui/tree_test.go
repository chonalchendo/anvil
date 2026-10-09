package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func treeVault(t *testing.T) (string, int) {
	t.Helper()
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeProductDesign, "p", map[string]any{"project": "p", "title": "P product"}, "x\n")
	writeArtifact(t, v, core.TypeSystemDesign, "p", map[string]any{"project": "p", "title": "P system"}, "x\n")
	writeArtifact(t, v, core.TypeComponentDesign, "p.linked", map[string]any{"project": "p", "title": "Linked comp", "system_design": "[[system-design.p]]"}, "x\n")
	writeArtifact(t, v, core.TypeComponentDesign, "p.loose", map[string]any{"project": "p", "title": "Loose comp"}, "x\n")
	writeArtifact(t, v, core.TypeComponentDesign, "p.dangling", map[string]any{"project": "p", "title": "Dangling comp", "system_design": "[[system-design.gone]]"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.finished", map[string]any{"project": "p", "title": "Finished ms", "status": "done", "product_design": "[[product-design.p]]"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.active", map[string]any{"project": "p", "title": "Active ms", "status": "in-progress", "product_design": "[[product-design.p]]"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.waiting", map[string]any{"project": "p", "title": "Waiting ms", "status": "planned", "product_design": "[[product-design.p]]"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.loose", map[string]any{"project": "p", "title": "Loose ms", "status": "planned"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0001-doing", map[string]any{"project": "p", "title": "Doing issue", "status": "in-progress", "milestone": "[[milestone.p.active]]"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0002-queued", map[string]any{"project": "p", "title": "Queued issue", "status": "open", "milestone": "[[milestone.p.active]]"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0000-shipped", map[string]any{"project": "p", "title": "Shipped issue", "status": "resolved", "milestone": "[[milestone.p.active]]"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.q.0001-orphan", map[string]any{"project": "q", "title": "Orphan project issue", "status": "open"}, "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0003-idle", map[string]any{"project": "p", "title": "Idle ms issue", "status": "in-progress", "milestone": "[[milestone.p.waiting]]"}, "x\n")
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Reindex(v.Root); err != nil {
		t.Fatal(err)
	}
	h, err := Handler(v, db)
	if err != nil {
		t.Fatal(err)
	}
	code, body := do(h, "GET", "/")
	return body, code
}

func inOrder(t *testing.T, body string, parts ...string) {
	t.Helper()
	at := 0
	for _, p := range parts {
		i := strings.Index(body[at:], p)
		if i < 0 {
			t.Fatalf("%q missing or out of order in:\n%s", p, body)
		}
		at += i + len(p)
	}
}

func TestTree_SpineOrder(t *testing.T) {
	body, code := treeVault(t)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	inOrder(t, body, "/artifact/product-design.p", "/artifact/system-design.p", "/artifact/component-design.p.linked", "Milestones · in-progress", "/artifact/milestone.p.active", "/artifact/issue.p.0001-doing", "Milestones · planned", "/artifact/milestone.p.waiting", "Milestones · done", "/artifact/milestone.p.finished")
	if !strings.Contains(body, "Doing issue") || !strings.Contains(body, "<details open>") {
		t.Fatalf("titles or folding missing:\n%s", body)
	}
}

func TestTree_UnlinkedUnderProject(t *testing.T) {
	body, _ := treeVault(t)
	inOrder(t, body, "Unlinked", "/artifact/component-design.p.loose")
	inOrder(t, body, "Unlinked", "/artifact/component-design.p.dangling")
	i := strings.Index(body, "Unlinked")
	if !strings.Contains(body[i:], "/artifact/milestone.p.loose") || strings.Contains(body[:i], "p.loose") {
		t.Fatalf("unlinked nodes misplaced:\n%s", body)
	}
}

func TestTree_InProgressMilestoneListsAllIssuesInStatusOrder(t *testing.T) {
	body, _ := treeVault(t)
	inOrder(t, body, "/artifact/milestone.p.active", "/artifact/issue.p.0001-doing", "/artifact/issue.p.0002-queued", "/artifact/issue.p.0000-shipped", "Milestones · planned")
	if strings.Contains(body, "issue.p.0003-idle") {
		t.Fatalf("issue under planned milestone rendered:\n%s", body)
	}
}

func TestTree_IssueOnlyProjectHasNoSection(t *testing.T) {
	body, _ := treeVault(t)
	if strings.Contains(body, "<h2>q</h2>") {
		t.Fatalf("issue-only project rendered:\n%s", body)
	}
}

// manyIssues seeds an in-progress milestone with n issues and returns the home body.
func manyIssues(t *testing.T, n int) string {
	t.Helper()
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeProductDesign, "p", map[string]any{"project": "p", "title": "P product"}, "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.big", map[string]any{"project": "p", "title": "Big ms", "status": "in-progress", "product_design": "[[product-design.p]]"}, "x\n")
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("issue.p.%04d-i", i)
		writeArtifact(t, v, core.TypeIssue, id, map[string]any{"project": "p", "title": fmt.Sprintf("Issue %d", i), "status": "open", "milestone": "[[milestone.p.big]]"}, "x\n")
	}
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Reindex(v.Root); err != nil {
		t.Fatal(err)
	}
	h, err := Handler(v, db)
	if err != nil {
		t.Fatal(err)
	}
	_, body := do(h, "GET", "/")
	return body
}

// Warrant: a group summary must show how many children it holds, so the count is the whole group not the shown rows.
func TestTreeCount_SummaryCarriesGroupSize(t *testing.T) {
	body := manyIssues(t, 10)
	inOrder(t, body, "Milestones · in-progress", `<span class="count">1</span>`, "Big ms", `<span class="count">10</span>`)
}

// Warrant: past eight rows the rest must fold behind one "N more" row, with no row lost or repeated.
func TestTreeMore_FoldsPastEight(t *testing.T) {
	body := manyIssues(t, 10)
	if got := strings.Count(body, `<li class="more">`); got != 1 {
		t.Fatalf("more rows = %d, want 1", got)
	}
	inOrder(t, body, "issue.p.0007-i", `<li class="more">`, "2 more", "issue.p.0008-i", "issue.p.0009-i")
	if strings.Count(body, "/artifact/issue.p.") != 10 {
		t.Fatalf("issue rows lost or repeated:\n%s", body)
	}
	if strings.Contains(manyIssues(t, 8), `class="more"`) {
		t.Fatal("a group of exactly eight folded")
	}
}

// Warrant: every page must end with the key hint bar, listing only keys the page handles.
func TestKeyHints_FooterOnEveryPage(t *testing.T) {
	body := manyIssues(t, 1)
	i := strings.Index(body, `<footer class="keys">`)
	if i < 0 || strings.Index(body, "</main>") > i {
		t.Fatalf("footer missing or before main:\n%s", body)
	}
	for _, k := range []string{">j<", ">k<", ">o<", ">⇧O<", ">⇧C<", ">⌘K<"} {
		if !strings.Contains(body[i:], k) {
			t.Errorf("hint bar lacks %s", k)
		}
	}
}
