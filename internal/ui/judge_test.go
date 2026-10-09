package ui

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func judgeStrip(t *testing.T, body string) string {
	t.Helper()
	_, rest, ok := strings.Cut(body, `<div class="judge">`)
	if !ok {
		return ""
	}
	strip, _, _ := strings.Cut(rest, "</div>")
	return strip
}

func TestJudgeStrip_LearningShowsSetFieldsAndLeavesProps(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeLearning, "judged", map[string]any{
		"title": "J", "confidence": "medium", "diataxis": "how-to", "tags": []any{"x"},
	}, "b\n")
	_, body := do(h, "GET", "/artifact/learning.judged")
	if !strings.Contains(contentsOf(t, body), `class="judge"`) {
		t.Fatal("judge fields missing from the contents column")
	}
	strip := judgeStrip(t, body)
	for _, want := range []string{">confidence<", ">medium<", ">diataxis<", ">how-to<"} {
		if !strings.Contains(strip, want) {
			t.Errorf("strip lacks %q", want)
		}
	}
	_, props, _ := strings.Cut(body, `<details class="props">`)
	if strings.Contains(props, "<dt>confidence</dt>") || strings.Contains(props, "<dt>diataxis</dt>") {
		t.Error("judge keys still in All properties")
	}
	if !strings.Contains(props, "<dt>tags</dt>") {
		t.Error("non-judge key left props")
	}
}

func TestJudgeStrip_NoFieldSetRendersNoStrip(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeLearning, "bare", map[string]any{"title": "Bare"}, "b\n")
	_, body := do(h, "GET", "/artifact/learning.bare")
	if strings.Contains(body, `class="judge"`) {
		t.Error("empty strip rendered")
	}
}

func TestJudgeStrip_OtherTypesGetNone(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeThread, "t-conf", map[string]any{"title": "T", "confidence": "high"}, "b\n")
	_, body := do(h, "GET", "/artifact/thread.t-conf")
	if strings.Contains(body, `class="judge"`) {
		t.Error("thread got a judge strip")
	}
	_, props, _ := strings.Cut(body, `<details class="props">`)
	if !strings.Contains(props, "<dt>confidence</dt>") {
		t.Error("non-judge type lost confidence from props")
	}
}

// The strip follows judgeKeys order: date, supersedes, superseded_by.
// A reorder of that slice must fail here, not pass on mere presence.
func TestJudgeStrip_DecisionShowsEveryKeyAndNoMeasured(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0002-dated", map[string]any{
		"title": "D", "date": "2026-10-09",
		"supersedes":    []any{"[[decision.ui.0001-a-decision]]"},
		"superseded_by": []any{"[[decision.ui.0001-a-decision]]"},
	}, "## Status\n\nMeasured: x\n")
	_, body := do(h, "GET", "/artifact/decision.ui.0002-dated")
	strip := judgeStrip(t, body)
	for _, want := range []string{">date<", ">2026-10-09<", ">supersedes<", `href="/artifact/decision.ui.0001-a-decision"`, ">superseded_by<"} {
		if !strings.Contains(strip, want) {
			t.Errorf("strip lacks %q", want)
		}
	}
	date, sup, supBy := strings.Index(strip, ">date<"), strings.Index(strip, ">supersedes<"), strings.Index(strip, ">superseded_by<")
	if date >= sup || sup >= supBy {
		t.Fatalf("strip order: date=%d supersedes=%d superseded_by=%d, want date < supersedes < superseded_by", date, sup, supBy)
	}
	if strings.Contains(strip, ">measured<") {
		t.Error("decision got a Measured row")
	}
}

func TestJudgeStrip_EmptyListRendersNoStrip(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeDecision, "ui.0004-empty", map[string]any{"title": "E", "supersedes": []any{}}, "b\n")
	_, body := do(h, "GET", "/artifact/decision.ui.0004-empty")
	if strings.Contains(body, `class="judge"`) {
		t.Error("empty list rendered a strip")
	}
}

func TestJudgeStrip_MilestoneShowsLastMeasuredFromStatusOnly(t *testing.T) {
	h, v := seed(t)
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.judged", map[string]any{"title": "M", "approved": "2026-10-09", "done": "2026-10-10"},
		"Measured: before status\n\n## Status\n\nMeasured: first\n\nMeasured: last `one`\n\n## Other\n\nMeasured: after status\n")
	_, body := do(h, "GET", "/artifact/milestone.milestone.anvil.judged")
	strip := judgeStrip(t, body)
	for _, want := range []string{">approved<", ">2026-10-09<", ">done<", ">2026-10-10<", ">measured<", ">last one<"} {
		if !strings.Contains(strip, want) {
			t.Errorf("strip lacks %q", want)
		}
	}
	for _, no := range []string{"first", "before status", "after status"} {
		if strings.Contains(strip, no) {
			t.Errorf("strip shows %q", no)
		}
	}
}

func TestLastMeasured_Boundaries(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"no status section":    {"Measured: x\n", ""},
		"status without line":  {"## Status\n\nnothing\n", ""},
		"last line of file":    {"## Status\nMeasured: end", "end"},
		"section ends":         {"## Status\nMeasured: a\n## Next\nMeasured: b\n", "a"},
		"status after other":   {"## Next\nMeasured: b\n## Status\nMeasured: a\n", "a"},
		"deeper heading stays": {"## Status\nMeasured: a\n### Sub\nMeasured: c\n", "c"},
	} {
		if got := lastMeasured(tc.body); got != tc.want {
			t.Errorf("%s: got %q want %q", name, got, tc.want)
		}
	}
}
