package ui

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func paletteOf(t *testing.T, body string) []paletteEntry {
	t.Helper()
	var got []paletteEntry
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("palette is not a JSON array of entries: %v", err)
	}
	return got
}

func typesOf(es []paletteEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Type
	}
	return out
}

// Warrant: without the weight sort a common word fills the list with low-value types before any project or milestone.
func TestPalette_RanksByTypeWeightThenRecency(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeIssue, "anvil.0002-newer", map[string]any{"title": "Newer", "project": "anvil", "updated": "2026-10-10"}, "x\n")
	_, body := do(h, "GET", "/palette")
	got := paletteOf(t, body)
	if got[0].Type != "project" || got[0].Href != "/project/anvil" {
		t.Errorf("first entry = %+v, want the anvil project", got[0])
	}
	var weights []int
	for _, e := range got {
		weights = append(weights, slices.Index(paletteWeight, e.Type))
	}
	if !slices.IsSorted(weights) || slices.Contains(weights, -1) {
		t.Errorf("types not in weight order: %v", typesOf(got))
	}
	var issues []string
	for _, e := range got {
		if e.Type == "issue" {
			issues = append(issues, e.Key)
		}
	}
	if len(issues) != 2 || !strings.HasSuffix(issues[0], "0002-newer") {
		t.Errorf("issues not newest first: %v", issues)
	}
}

// Warrant: a topic missing here, or a session kept, defeats decision 0004's palette rule.
func TestPalette_ListsTopicsAndDropsSessions(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeSession, "s-1", map[string]any{"title": "A session"}, "x\n")
	_, body := do(h, "GET", "/palette")
	got := paletteOf(t, body)
	var topic, decision bool
	for _, e := range got {
		switch {
		case e.Type == "session":
			t.Errorf("palette holds a session: %+v", e)
		case e.Type == "topic" && e.Key == "ui":
			topic = e.Href == "/topic/ui"
		case e.Type == "decision" && e.Title == "A decision":
			decision = e.Href == "/artifact/decision.ui.0001-a-decision"
		}
	}
	if !topic || !decision {
		t.Errorf("topic=%v decision=%v in %s", topic, decision, body)
	}
}

// Warrant: a button without aria-haspopup, a list without aria-live, or a Search row gated on zero matches breaks the mockup's rules.
func TestPalette_OpenerAndSearchRow(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/")
	for _, want := range []string{
		`<button type="button" class="search" id="palette-q" aria-haspopup="dialog"`,
		`<ul id="palette-list" aria-live="polite">`,
		`if (entries && q.value.trim()) {`,
		`location.href = items[cur].dataset.href;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}
