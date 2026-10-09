package ui

import (
	"strings"
	"testing"
)

func TestNodeHeader_ShowsIdentityAndSlots(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/issue."+stackIssue)
	i := strings.Index(body, `<header class="node">`)
	j := strings.Index(body, `</header>`)
	if i < 0 || j < i {
		t.Fatal("node header missing")
	}
	head := body[i:j]
	for _, want := range []string{"<h1>Thing</h1>", `class="status in-progress"`, "●", "in-progress", `href="/artifact/milestone.anvil.m1"`} {
		if !strings.Contains(head, want) {
			t.Errorf("header lacks %q", want)
		}
	}
}

func TestProps_FoldedClosedAndHeaderKeysExcluded(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/issue."+stackIssue)
	if !strings.Contains(body, `<details class="props">`) || strings.Contains(body, `<details class="props" open`) {
		t.Fatal("props must fold closed")
	}
	props := body[strings.Index(body, `<details class="props">`):]
	props = props[:strings.Index(props, "</details>")]
	if !strings.Contains(props, "<dt>learnings</dt>") {
		t.Error("learnings missing from props")
	}
	for _, no := range []string{"<dt>title</dt>", "<dt>status</dt>", "<dt>milestone</dt>"} {
		if strings.Contains(props, no) {
			t.Errorf("props repeats header field %s", no)
		}
	}
}
