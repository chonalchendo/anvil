package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/hydrate"
)

const stackIssue = "issue.anvil.0001.thing"

func TestStack_OrderMatchesHydrate(t *testing.T) {
	h, v := seed(t)
	code, body := do(h, "GET", stackPath)
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

// Fails if a layer's size is dropped, zeroed or shown on the wrong layer.
func TestStack_LayerSizes(t *testing.T) {
	h, v := seed(t)
	_, body := do(h, "GET", stackPath)
	hy, err := hydrate.Assemble(v, stackIssue)
	if err != nil {
		t.Fatal(err)
	}
	summaries := regexp.MustCompile(`(?s)<summary>(.*?)</summary>`).FindAllStringSubmatch(body, -1)
	if len(summaries) != len(hy.Nodes) {
		t.Fatalf("summaries = %d, want %d", len(summaries), len(hy.Nodes))
	}
	want := map[string]string{stackIssue: "2.0 KB", "learning.a-learning": "4.0 KB"}
	for i, n := range hy.Nodes {
		key := core.IndexKey(n.Type, n.ID)
		sum := summaries[i][1]
		if !strings.Contains(sum, kb(len(n.Body))) {
			t.Errorf("%s summary lacks its size %s: %s", key, kb(len(n.Body)), sum)
		}
		if w, ok := want[key]; ok {
			if !strings.Contains(sum, w) {
				t.Errorf("%s summary lacks %s: %s", key, w, sum)
			}
			delete(want, key)
		}
	}
	if len(want) > 0 {
		t.Errorf("layers missing from stack: %v", want)
	}
}

func TestStack_UnknownIssue404(t *testing.T) {
	h, _ := seed(t)
	for _, p := range []string{"/issue/issue.anvil.9999.nope/stack", "/issue/" + stackIssue + "x/stack", "/issue/milestone.anvil.m1/stack"} {
		if code, _ := do(h, "GET", p); code != 404 {
			t.Errorf("GET %s = %d, want 404", p, code)
		}
	}
}

func TestStack_TabOnIssuePageOnly(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/"+stackIssue)
	if !strings.Contains(body, `href="/issue/`+stackIssue+`/stack"`) {
		t.Error("issue page has no Stack tab")
	}
	_, body = do(h, "GET", decisionPath)
	if strings.Contains(body, "/stack") {
		t.Error("non-issue page has a Stack tab")
	}
}
