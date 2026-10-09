package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestTypeList_FiltersDiscriminate(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "other.0001-elsewhere", map[string]any{
		"title": "Elsewhere", "project": "other", "status": "superseded",
	}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0002-second", map[string]any{
		"title": "Second one", "project": "anvil", "status": "accepted",
	}, "x\n")
	const first, other, second = "/artifact/decision.ui.0001-a-decision", "/artifact/decision.other.0001-elsewhere", "/artifact/decision.ui.0002-second"
	cases := []struct {
		query         string
		want, exclude []string
	}{
		{"", []string{first, other, second}, nil},
		{"?project=other", []string{other}, []string{first, second}},
		{"?status=accepted", []string{second}, []string{first, other}},
		{"?project=anvil&status=accepted", []string{second}, []string{first, other}},
	}
	for _, c := range cases {
		code, body := do(h, "GET", "/type/decision"+c.query)
		if code != 200 {
			t.Fatalf("%q = %d", c.query, code)
		}
		for _, w := range c.want {
			if !strings.Contains(body, w) {
				t.Errorf("%q lacks %s", c.query, w)
			}
		}
		for _, x := range c.exclude {
			if strings.Contains(body, x) {
				t.Errorf("%q should not list %s", c.query, x)
			}
		}
	}
	if _, body := do(h, "GET", "/type/decision?status=accepted"); !strings.Contains(body, "Second one") {
		t.Error("row lacks its title")
	}
}

// Warrant: if the comparator ignored status or updated, groups would repeat or rows misorder.
func TestTypeList_GroupsLiveFirstNewestFirst(t *testing.T) {
	h, v := seed(t)
	for id, fm := range map[string][2]string{
		"ui.0010-older": {"accepted", "2026-01-01"},
		"ui.0011-newer": {"accepted", "2026-03-01"},
		"ui.0012-live":  {"proposed", "2026-02-01"},
		"ui.0013-dead":  {"rejected", "2026-04-01"},
		"ui.0014-gone":  {"stale", "2026-04-02"},
		"ui.0015-odd":   {"weird", "2026-04-03"},
	} {
		writeArtifact(t, v, core.TypeDecision, id, map[string]any{
			"title": id, "project": "anvil", "status": fm[0], "updated": fm[1],
		}, "x\n")
	}
	_, body := do(h, "GET", "/type/decision")
	at := func(id string) int { return strings.Index(body, "/artifact/decision."+id) }
	if at("ui.0011-newer") > at("ui.0010-older") {
		t.Error("older decision listed before newer in one group")
	}
	for _, c := range [][2]string{
		{"ui.0012-live", "ui.0010-older"},
		{"ui.0010-older", "ui.0014-gone"},
		{"ui.0014-gone", "ui.0013-dead"},
		{"ui.0013-dead", "ui.0015-odd"},
	} {
		if at(c[0]) > at(c[1]) {
			t.Errorf("%s should precede %s", c[0], c[1])
		}
	}
	for _, st := range []string{"accepted", "proposed", "rejected", "stale", "weird"} {
		if n := strings.Count(body, `aria-label="`+st+`"`); n != 1 {
			t.Errorf("status heading %q appears %d times, want 1", st, n)
		}
	}
	for _, st := range []string{"accepted", "proposed", "rejected", "stale"} {
		if glyphs[st] == "" {
			t.Errorf("no glyph for seeded status %s", st)
		}
	}
}

func TestTypeList_UnknownType404(t *testing.T) {
	h, _ := seed(t)
	if code, _ := do(h, "GET", "/type/nonsense"); code != 404 {
		t.Errorf("unknown type = %d, want 404", code)
	}
}

func TestPalette_JSONShape(t *testing.T) {
	h, _ := seed(t)
	code, body := do(h, "GET", "/palette")
	if code != 200 {
		t.Fatalf("palette = %d", code)
	}
	var got []map[string]string
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("palette is not a JSON array of strings: %v", err)
	}
	var found bool
	for _, e := range got {
		if len(e) != 4 || e["key"] == "" || e["type"] == "" || e["title"] == "" {
			t.Errorf("bad entry %v", e)
		}
		if e["key"] == "decision.ui.0001-a-decision" {
			found = e["type"] == "decision" && e["title"] == "A decision"
		}
	}
	if !found {
		t.Error("palette lacks the seeded decision")
	}
}
