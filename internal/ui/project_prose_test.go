package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func section(body, id string) string {
	_, rest, _ := strings.Cut(body, `id="`+id+`"`)
	out, _, _ := strings.Cut(rest, "</section>")
	return out
}

// Warrant: fails if a knowledge band falls back to a list or loses its dated, linked title heads.
func TestProse_DecidedAndLearnedReadAsSentences(t *testing.T) {
	h, v := seed(t)
	for i, st := range []string{"proposed", "proposed", "proposed", "accepted"} {
		writeArtifact(t, v, core.TypeDecision, fmt.Sprintf("anvil.%04d-d", i+1), map[string]any{
			"title": fmt.Sprintf("Decision %d", i+1), "status": st, "project": "anvil", "updated": fmt.Sprintf("2026-10-0%d", i+1),
		}, "x\n")
	}
	for i, c := range []string{"high", "low", "high", "medium"} {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("anvil-l%d", i), map[string]any{
			"title": fmt.Sprintf("Learning %d", i), "status": "draft", "project": "anvil", "confidence": c, "updated": fmt.Sprintf("2026-10-0%d", i+1),
		}, "x\n")
	}
	writeArtifact(t, v, core.TypeLearning, "anvil-checked", map[string]any{"title": "Checked", "status": "verified", "project": "anvil", "updated": "2026-09-01"}, "x\n")
	body, _ := projectBody(t, h)
	decided, learned := section(body, "decided"), section(body, "learned")
	for _, want := range []string{
		"3 proposals wait on you. Newest:", `<a href="/artifact/decision.anvil.0003-d" class="to-proposed">Decision 3</a> (<time datetime="2026-10-03">3 Oct</time>)`,
		"Older:", "Accepted last:", `class="to-accepted">Decision 4</a>`,
	} {
		if !strings.Contains(decided, want) {
			t.Errorf("decided band lacks %q in\n%s", want, decided)
		}
	}
	for _, want := range []string{
		"4 drafts and 1 verified; nothing new since <time datetime=\"2026-10-04\">4 Oct</time>.",
		"Newest drafts:", "at medium confidence", "Held at high confidence, unverified:", "Verified:", ">Checked</a>",
	} {
		if !strings.Contains(learned, want) {
			t.Errorf("learned band lacks %q in\n%s", want, learned)
		}
	}
	first, rest, _ := strings.Cut(learned, "</p>")
	if !strings.Contains(first, "Newest drafts:") || !strings.Contains(rest, "Held at high confidence") || strings.Contains(rest, "Newest drafts:") {
		t.Errorf("Newest clause must close paragraph one and Held open paragraph two in\n%s", learned)
	}
	if n := strings.Count(learned, `>Learning 2</a>`); n != 1 {
		t.Errorf("the newest high-confidence draft is linked %d times, want 1", n)
	}
	for name, band := range map[string]string{"decided": decided, "learned": learned} {
		if strings.Contains(band, "<ul") || strings.Contains(band, "<li") {
			t.Errorf("%s band holds a list", name)
		}
	}
}

// Warrant: fails if the Decided band's counts and names disagree, or its overflow link or the Learned band's shared confidence slip.
func TestProse_OverflowAndSharedConfidence(t *testing.T) {
	h, v := seed(t)
	for i := 1; i <= 7; i++ {
		writeArtifact(t, v, core.TypeDecision, fmt.Sprintf("anvil.%04d-d", i), map[string]any{
			"title": fmt.Sprintf("Decision %d", i), "status": "proposed", "project": "anvil", "updated": fmt.Sprintf("2026-10-0%d", i),
		}, "x\n")
	}
	for i := 0; i < 2; i++ {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("anvil-l%d", i), map[string]any{
			"title": fmt.Sprintf("Learning %d", i), "status": "draft", "project": "anvil", "confidence": "low", "updated": fmt.Sprintf("2026-10-0%d", i+1),
		}, "x\n")
	}
	body, _ := projectBody(t, h)
	decided, learned := section(body, "decided"), section(body, "learned")
	for _, want := range []string{"7 proposals wait on you.", "Newest:", "Older:", `; <a href="/type/decision?project=anvil&amp;status=proposed">3 more</a>.`} {
		if !strings.Contains(decided, want) {
			t.Errorf("decided band lacks %q in\n%s", want, decided)
		}
	}
	for _, want := range []string{"2 drafts and none verified; nothing new since", "Newest, both at low confidence:"} {
		if !strings.Contains(learned, want) {
			t.Errorf("learned band lacks %q in\n%s", want, learned)
		}
	}
	if strings.Contains(learned, "at low confidence</") || strings.Contains(learned, ") at low") {
		t.Errorf("shared confidence is repeated per title in\n%s", learned)
	}
}

// Warrant: fails if a page drops the skip link or the palette stops naming its selected row to a screen reader.
func TestBase_SkipLinkAndComboboxPalette(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/project/anvil")
	for _, want := range []string{`<a class="skip" href="#main">`, `<main id="main">`, `role="combobox"`, `aria-controls="palette-list"`, `aria-activedescendant`, `role="listbox"`, `aria-expanded="false"`, `id="palette-status"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}

// Warrant: fails if a text size leaves the 11/13/16/19/28 scale (mono 12) or links lose the muted underline.
func TestCSS_OneTypeScaleAndMutedLinks(t *testing.T) {
	css := cssSource(t)
	for _, m := range regexp.MustCompile(`font(?:-size)?:\s*(?:[0-9.]+(?:px)?\s+)?([0-9.]+)px`).FindAllStringSubmatch(css, -1) {
		switch m[1] {
		case "11", "12", "13", "16", "19", "28":
		default:
			t.Errorf("font size %spx is off the scale: %s", m[1], m[0])
		}
	}
	for _, want := range []string{"--line: #45494f", "text-decoration-color: var(--line)", "a:hover { text-decoration-color: var(--hue, var(--text))", ".skip:focus", "[id] { scroll-margin-top"} {
		if !strings.Contains(css, want) {
			t.Errorf("css lacks %q", want)
		}
	}
	if strings.Contains(css, "transition: all") {
		t.Error("css holds transition: all")
	}
}
