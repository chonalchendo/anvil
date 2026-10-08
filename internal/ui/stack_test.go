package ui

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/hydrate"
	"github.com/chonalchendo/anvil/internal/index"
)

const stackIssue = "issue.anvil.0001.thing"

func seedStack(t *testing.T) (http.Handler, *core.Vault) {
	t.Helper()
	_, v := seed(t)
	writeArtifact(t, v, core.TypeIssue, stackIssue, map[string]any{
		"title": "Thing", "status": "in-progress",
		"milestone": "[[milestone.anvil.m1]]",
		"learnings": []any{"[[learning.a-learning]]", "[[learning.ghost]]"},
	}, "issue body\n\n## Links\n\n- [[thread.anvil-design-docs.0002-x]]\n")
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
	return h, v
}

func TestStack_OrderMatchesHydrate(t *testing.T) {
	h, v := seedStack(t)
	code, body := do(h, "GET", "/issue/"+stackIssue+"/stack")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	hy, err := hydrate.Assemble(v, stackIssue)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, n := range hy.Nodes {
		want = append(want, `href="/artifact/`+core.IndexKey(n.Type, n.ID)+`"`)
	}
	at := 0
	for _, w := range want {
		i := strings.Index(body[at:], w)
		if i < 0 {
			t.Fatalf("%s missing or out of order in page", w)
		}
		at += i + len(w)
	}
	if got := strings.Count(body, `<details class="layer">`); got != len(want) {
		t.Errorf("layers = %d, want %d", got, len(want))
	}
	if !strings.Contains(body, "learning.ghost") || !strings.Contains(body, "Broken edges") {
		t.Errorf("broken edge not shown")
	}
	if !strings.Contains(body, "Skipped body links") {
		t.Errorf("skipped body link not shown")
	}
}

func TestStack_LayerSizes(t *testing.T) {
	h, v := seedStack(t)
	_, body := do(h, "GET", "/issue/"+stackIssue+"/stack")
	hy, err := hydrate.Assemble(v, stackIssue)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range hy.Nodes {
		if want := kb(len(n.Body)); !strings.Contains(body, want) {
			t.Errorf("size %s of %s not on page", want, n.ID)
		}
	}
	if !regexp.MustCompile(`\d+\.\d KB`).MatchString(body) {
		t.Error("no size rendered")
	}
}

func TestStack_UnknownIssue404(t *testing.T) {
	h, _ := seedStack(t)
	for _, p := range []string{"/issue/issue.anvil.9999.nope/stack", "/issue/" + stackIssue + "x/stack", "/issue/milestone.anvil.m1/stack"} {
		if code, _ := do(h, "GET", p); code != 404 {
			t.Errorf("GET %s = %d, want 404", p, code)
		}
	}
	if code, _ := do(h, "POST", "/issue/"+stackIssue+"/stack"); code != 405 {
		t.Errorf("POST = %d, want 405", code)
	}
}

func TestStack_TabOnIssuePageOnly(t *testing.T) {
	h, _ := seedStack(t)
	_, body := do(h, "GET", "/artifact/"+stackIssue)
	if !strings.Contains(body, `href="/issue/`+stackIssue+`/stack"`) {
		t.Error("issue page has no Stack tab")
	}
	_, body = do(h, "GET", decisionPath)
	if strings.Contains(body, "/stack") {
		t.Error("non-issue page has a Stack tab")
	}
}
