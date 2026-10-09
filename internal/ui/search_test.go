package ui

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// Warrant: fails if /search skips a body outside issue/milestone, lets body
// text inject markup, or drops the "no matches" or empty-form states.
func TestSearchPage(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeLearning, "found-by-body", map[string]any{"title": "Body only"},
		"## TL;DR\n\nthe quokka habitat <b>matters</b>\n")
	// seed built the index before this write; the fresh() throttle reindexes on the first request.
	code, body := do(h, "GET", "/search?q=quokka")
	if code != 200 {
		t.Fatalf("/search = %d", code)
	}
	for _, want := range []string{`href="/artifact/learning.found-by-body"`, `<mark>quokka</mark>`, `&lt;b&gt;matters&lt;/b&gt;`} {
		if !strings.Contains(body, want) {
			t.Errorf("search body lacks %q", want)
		}
	}
	if strings.Contains(body, "<b>matters") {
		t.Error("body text injected markup into the snippet")
	}
	_, none := do(h, "GET", "/search?q=nonexistentterm")
	if !strings.Contains(none, "no matches") {
		t.Error("a query with no hits lacks 'no matches'")
	}
	_, empty := do(h, "GET", "/search")
	if strings.Contains(empty, "no matches") || !strings.Contains(empty, `name="q"`) {
		t.Error("empty q must render the form only")
	}
}

// Warrant: fails if a hostile query (NUL byte, lone quote, trailing FTS operator) reaches FTS5 unsanitised and 500s.
func TestSearchPage_HostileQueries(t *testing.T) {
	h, _ := seed(t)
	for _, q := range []string{"%00", "%22", "a%20OR", "%00%20%00", "NEAR(", "a*", strings.Repeat("a", 60<<10)} {
		if code, _ := do(h, "GET", "/search?q="+q); code != 200 {
			t.Errorf("/search?q=%s = %d, want 200", q, code)
		}
	}
}
