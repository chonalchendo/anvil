package ui

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// Warrant: fails if an unknown slug stops being a 404 or a known one stops rendering the reading page.
func TestTopic_Status(t *testing.T) {
	h := topicVault(t)
	if code, _ := do(h, "GET", "/topic/no-such-topic"); code != 404 {
		t.Errorf("unknown topic = %d, want 404", code)
	}
	if code, _ := do(h, "GET", "/topic/alpha"); code != 200 {
		t.Errorf("known topic = %d, want 200", code)
	}
}

// Warrant: fails if the page loses its breadcrumb, composed lead, ordinal order, descriptions or All links.
func TestTopic_DecisionsAndThreads(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/topic/alpha")
	inOrder(t, body, `<nav class="crumbs"`, `href="/">Knowledge</a>`, `<h1 translate="no">alpha</h1>`,
		"3 decisions (2 accepted, 1 superseded) and 1 thread (1 open). Last moved 9 Oct.",
		"Decisions", `href="/type/decision?topic=alpha">All 3`,
		"0001", "Old alpha line.", "0002", "Newest alpha line.", "0003", "Superseded line.",
		"Threads", `href="/type/thread?topic=alpha">All 1`, "Is it so", "Learnings", "Raw inbox")
}

// Warrant: fails if a linked learning is missed, a tag match leaks in unrelated learnings, or a linked one repeats under the tag group.
func TestTopic_Learnings(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/topic/alpha")
	linked, tagged, _ := strings.Cut(body, "By domain tag")
	if !strings.Contains(linked, "Linked learning") || strings.Contains(linked, "Tagged learning") {
		t.Errorf("linked group wrong:\n%s", linked)
	}
	tagged, _, _ = strings.Cut(tagged, "Raw inbox")
	if !strings.Contains(tagged, "Tagged learning") || strings.Contains(tagged, "Other learning") || strings.Contains(tagged, "Linked learning") {
		t.Errorf("tag group wrong:\n%s", tagged)
	}
	_, none := do(topicVault(t), "GET", "/topic/gamma")
	if !strings.Contains(none, "None linked yet") {
		t.Error("empty linked group lacks its note")
	}
}

// Warrant: fails if a raw note naming the slug or linking a member by related is missed, a note not naming it appears, or the page lists a non-raw note.
func TestTopic_RawInbox(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/topic/alpha")
	_, inbox, _ := strings.Cut(body, `id="inb"`)
	if !strings.Contains(inbox, "Cites alpha") || !strings.Contains(inbox, "Links by related") || strings.Contains(inbox, "Plain note") || strings.Contains(inbox, "Routed note") {
		t.Errorf("inbox panel wrong:\n%s", inbox)
	}
}

// Warrant: fails if more than tagCap tag matches render or the "N more" link stops pointing at the tag filter.
func TestTopic_TagCap(t *testing.T) {
	v := newTagVault(t, tagCap+3)
	_, body := do(serve(t, v), "GET", "/topic/alpha")
	if n := strings.Count(body, "/artifact/learning.l"); n != tagCap {
		t.Errorf("tag learnings = %d, want %d", n, tagCap)
	}
	if !strings.Contains(body, `href="/type/learning?tag=domain%2Fui">3 more`) {
		t.Errorf("missing N more link:\n%s", body)
	}
}

// Warrant: fails if a path segment or a longer hyphenated word counts as naming the slug, or a real mention is missed.
func TestTopic_RawInboxWholeWord(t *testing.T) {
	v := newTagVault(t, 0)
	writeArtifact(t, v, core.TypeInbox, "2026-10-01-path", map[string]any{"title": "Path note", "status": "raw"}, "See internal/alpha/x.go and alpha-beta.\n")
	writeArtifact(t, v, core.TypeInbox, "2026-10-02-word", map[string]any{"title": "Word note", "status": "raw"}, "The alpha. topic moved.\n")
	_, body := do(serve(t, v), "GET", "/topic/alpha")
	_, inbox, _ := strings.Cut(body, `id="inb"`)
	if strings.Contains(inbox, "Path note") || !strings.Contains(inbox, "Word note") {
		t.Errorf("whole-word match wrong:\n%s", inbox)
	}
}

// Warrant: fails if "N more" links an unfiltered learning list when the topic carries more than one domain tag.
func TestTopic_TagCapManyTags(t *testing.T) {
	v := newTagVault(t, tagCap+2)
	writeArtifact(t, v, core.TypeThread, "alpha.0001-q", map[string]any{"title": "Q", "status": "open", "tags": []any{"domain/db"}}, "x\n")
	_, body := do(serve(t, v), "GET", "/topic/alpha")
	if !strings.Contains(body, ">2 more<") || strings.Contains(body, "2 more</a>") {
		t.Errorf("N more should be plain text:\n%s", body)
	}
}

// Warrant: fails if a decision whose description ends in "?" or "!" gains a full stop or joins its open thread with a semicolon, a thread-only topic loses the capitalised "Open: " lead, a decision-only topic loses its full stop, or a topic with nothing current renders an empty line.
func TestTopic_Stands(t *testing.T) {
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeThread, "solo.0001-q", map[string]any{"title": "Why so", "status": "open", "updated": "2026-10-02"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "solo.0002-r", map[string]any{"title": "Done one", "status": "resolved", "updated": "2026-10-01"}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "dec.0001-a", map[string]any{"title": "Dec A", "status": "accepted", "updated": "2026-10-01", "description": "Plain line."}, "x\n")
	writeArtifact(t, v, core.TypeThread, "dec.0001-q", map[string]any{"title": "Dec Q", "status": "resolved", "updated": "2026-10-01"}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "dead.0001-a", map[string]any{"title": "Old call", "status": "superseded", "updated": "2026-10-03", "description": "Gone."}, "x\n")
	writeArtifact(t, v, core.TypeThread, "dead.0001-q", map[string]any{"title": "Shut", "status": "resolved", "updated": "2026-10-02"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "dead.0002-q", map[string]any{"title": "Why dead?", "status": "resolved", "updated": "2026-10-04"}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "ask.0001-a", map[string]any{"title": "Ask", "status": "accepted", "updated": "2026-10-05", "description": "Why this?"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "ask.0001-q", map[string]any{"title": "Ask Q", "status": "resolved", "updated": "2026-10-05"}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "q.0001-a", map[string]any{"title": "T", "status": "accepted", "updated": "2026-10-01", "description": "Is it so?"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "q.0001-open", map[string]any{"title": "Still asking", "status": "open", "updated": "2026-10-02"}, "x\n")
	writeArtifact(t, v, core.TypeDecision, "bang.0001-a", map[string]any{"title": "T", "status": "accepted", "updated": "2026-10-01", "description": "Stop it!"}, "x\n")
	writeArtifact(t, v, core.TypeThread, "bang.0001-open", map[string]any{"title": "Still going", "status": "open", "updated": "2026-10-02"}, "x\n")
	_, body := do(serve(t, v), "GET", "/")
	if strings.Contains(body, "?; open") || strings.Contains(body, "!; open") {
		t.Errorf("terminal description joined with a semicolon:\n%s", body)
	}
	for _, want := range []string{`<p class="syn">Is it so? Open: Still asking</p>`, `<p class="syn">Stop it! Open: Still going</p>`, `<p class="syn">Why this?</p>`, `<p class="syn">Open: Why so</p>`, `<p class="syn">Plain line.</p>`, `<p class="syn">Nothing current; newest: Why dead?</p>`} {
		if !strings.Contains(body, want) {
			t.Errorf("knowledge page lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `<p class="syn"></p>`) {
		t.Error("empty where-it-stands line")
	}
}

// Warrant: fails if a topic's learnings or inbox lists stop rendering as an unordered list, or its threads stop rendering ordered.
func TestTopic_ListKinds(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/topic/alpha")
	if !strings.Contains(body, `<ul class="rows">`) || !strings.Contains(body, `<ol class="rows">`) || strings.Contains(body, `<span class="ord" translate="no"></span>`) {
		t.Errorf("newest-first lists wrong:\n%s", body)
	}
}

// Warrant: fails if a topic without a domain tag renders a comma after "By domain tag".
func TestTopic_NoTagNoComma(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/topic/gamma")
	if strings.Contains(body, "By domain tag,") || !strings.Contains(body, "By domain tag</h3>") {
		t.Errorf("tag heading wrong:\n%s", body)
	}
}
