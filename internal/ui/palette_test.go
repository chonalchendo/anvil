package ui

import (
	"encoding/json"
	"regexp"
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

// Warrant: a button without aria-haspopup or a list without aria-live breaks the mockup's rules.
func TestPalette_OpenerAndList(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/")
	button := regexp.MustCompile(`<button\b[^>]*\bid="palette-q"[^>]*>`).FindString(body)
	for _, attr := range []string{`type="button"`, `id="palette-q"`, `aria-haspopup="dialog"`} {
		if !strings.Contains(button, attr) {
			t.Errorf("opener %q lacks %s", button, attr)
		}
	}
	ul := regexp.MustCompile(`<ul\b[^>]*id="palette-list"[^>]*>|<ul\b[^>]*aria-live[^>]*>`).FindString(body)
	for _, attr := range []string{`id="palette-list"`, `aria-live="polite"`} {
		if !strings.Contains(ul, attr) {
			t.Errorf("list %q lacks %s", ul, attr)
		}
	}
}

// Warrant: an entry with an empty key, title or href renders a dead row.
func TestPalette_EntriesAreComplete(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/palette")
	for _, e := range paletteOf(t, body) {
		if e.Key == "" || e.Title == "" || e.Href == "" {
			t.Errorf("incomplete entry: %+v", e)
		}
	}
}

// Warrant: a type missing from paletteWeight sorts above projects silently.
func TestPalette_WeightCoversEveryType(t *testing.T) {
	for _, typ := range core.AllTypes {
		if typ == core.TypeSession {
			continue
		}
		if !slices.Contains(paletteWeight, string(typ)) {
			t.Errorf("type %q has no paletteWeight", typ)
		}
	}
}
