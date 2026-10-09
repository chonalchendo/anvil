package ui

import (
	"strings"
	"testing"
)

const comparePath = "/compare?a=product-design.anvil&b=decision.ui.0001-a-decision"

const pane = `<article class="pane">`

// Warrant: fails if /compare renders fewer or more than two panes, swaps the
// a/b order, drops a pane's node header or body, or adds the contents column.
func TestComparePage_TwoPanes(t *testing.T) {
	h, _ := seed(t)
	code, body := do(h, "GET", comparePath)
	if code != 200 {
		t.Fatalf("/compare = %d", code)
	}
	if n := strings.Count(body, pane); n != 2 {
		t.Fatalf("panes = %d, want 2", n)
	}
	if n := strings.Count(body, `<header class="node">`); n != 2 {
		t.Errorf("node headers = %d, want 2", n)
	}
	parts := strings.Split(body, pane)
	pa, pb := parts[1], parts[2]
	if !strings.Contains(pa, "<h1>Anvil product</h1>") || !strings.Contains(pa, "<p>product</p>") || strings.Contains(pa, "A decision") {
		t.Error("first pane is not the a node")
	}
	if !strings.Contains(pb, "<h1>A decision</h1>") || !strings.Contains(pb, "alert(1)") || strings.Contains(pb, "<h1>Anvil product</h1>") {
		t.Error("second pane is not the b node")
	}
	if !strings.Contains(pb, `<span>related <a href="/artifact/product-design.anvil"`) {
		t.Error("second pane lacks its related slot")
	}
	_, withCrumbs := do(h, "GET", "/compare?a=milestone.anvil.m1&b=product-design.anvil")
	if !strings.Contains(strings.Split(withCrumbs, pane)[1], `class="crumbs"`) {
		t.Error("a pane lost its breadcrumb")
	}
	if !strings.Contains(body, "<kbd>j</kbd>") {
		t.Error("compare must show the j/k key hints its sections respond to")
	}
	if strings.Contains(body, `class="contents"`) || !strings.Contains(body, `class="sidebar"`) {
		t.Error("compare must keep the sidebar and render no contents column")
	}
	_, swapped := do(h, "GET", "/compare?a=decision.ui.0001-a-decision&b=product-design.anvil")
	if !strings.Contains(strings.Split(swapped, pane)[1], "A decision") {
		t.Error("swapping a and b did not swap the panes")
	}
}

// Warrant: fails if a missing, empty, malformed or unknown key 500s or renders
// a half page, or if a == b stops rendering two panes.
func TestComparePage_EdgeKeys(t *testing.T) {
	h, _ := seed(t)
	for _, q := range []string{"", "?a=product-design.anvil", "?b=product-design.anvil", "?a=product-design.anvil&b=thread.nope", "?a=bogus.x&b=product-design.anvil", "?a=product-design.anvil&b=product-design.", "?a=product-design.anvil&b=../x"} {
		if code, _ := do(h, "GET", "/compare"+q); code != 404 {
			t.Errorf("GET /compare%s = %d, want 404", q, code)
		}
	}
	code, body := do(h, "GET", "/compare?a=product-design.anvil&b=product-design.anvil")
	if code != 200 || strings.Count(body, pane) != 2 {
		t.Errorf("a==b: status %d, panes %d, want 200 and 2", code, strings.Count(body, pane))
	}
}
