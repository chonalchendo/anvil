package ui

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func treeVault(t *testing.T) (string, int) {
	t.Helper()
	v := &core.Vault{Root: t.TempDir()}
	p := map[string]any{"project": "p"}
	with := func(fm map[string]any) map[string]any {
		fm["project"] = p["project"]
		return fm
	}
	writeArtifact(t, v, core.TypeProductDesign, "p", with(map[string]any{"title": "P product"}), "x\n")
	writeArtifact(t, v, core.TypeSystemDesign, "p", with(map[string]any{"title": "P system"}), "x\n")
	writeArtifact(t, v, core.TypeComponentDesign, "p.linked", with(map[string]any{"title": "Linked comp", "system_design": "[[system-design.p]]"}), "x\n")
	writeArtifact(t, v, core.TypeComponentDesign, "p.loose", with(map[string]any{"title": "Loose comp"}), "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.active", with(map[string]any{"title": "Active ms", "status": "in-progress", "product_design": "[[product-design.p]]"}), "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.waiting", with(map[string]any{"title": "Waiting ms", "status": "open", "product_design": "[[product-design.p]]"}), "x\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.p.loose", with(map[string]any{"title": "Loose ms", "status": "open"}), "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0001-doing", with(map[string]any{"title": "Doing issue", "status": "in-progress", "milestone": "[[milestone.p.active]]"}), "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0002-queued", with(map[string]any{"title": "Queued issue", "status": "open", "milestone": "[[milestone.p.active]]"}), "x\n")
	writeArtifact(t, v, core.TypeIssue, "issue.p.0003-idle", with(map[string]any{"title": "Idle ms issue", "status": "in-progress", "milestone": "[[milestone.p.waiting]]"}), "x\n")
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
	inOrder(t, body, "/artifact/product-design.p", "/artifact/system-design.p", "/artifact/component-design.p.linked", "Milestones · in-progress", "/artifact/milestone.p.active", "/artifact/issue.p.0001-doing", "Milestones · open", "/artifact/milestone.p.waiting")
	if !strings.Contains(body, "Doing issue") || !strings.Contains(body, "<details open>") {
		t.Fatalf("titles or folding missing:\n%s", body)
	}
}

func TestTree_UnlinkedUnderProject(t *testing.T) {
	body, _ := treeVault(t)
	inOrder(t, body, "Unlinked", "/artifact/component-design.p.loose")
	i := strings.Index(body, "Unlinked")
	if !strings.Contains(body[i:], "/artifact/milestone.p.loose") || strings.Contains(body[:i], "p.loose") {
		t.Fatalf("unlinked nodes misplaced:\n%s", body)
	}
}

func TestTree_InProgressIssuesOnly(t *testing.T) {
	body, _ := treeVault(t)
	for _, k := range []string{"issue.p.0002-queued", "issue.p.0003-idle"} {
		if strings.Contains(body, k) {
			t.Fatalf("%s must not appear", k)
		}
	}
}
