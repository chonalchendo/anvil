package ui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// serve indexes v and returns its handler.
func serve(t *testing.T, v *core.Vault) http.Handler {
	t.Helper()
	db, err := index.Open(index.DBPath(v.Root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Reindex(v.Root); err != nil {
		t.Fatal(err)
	}
	h, err := Handler(v, db)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// topicVault seeds topic alpha (two decisions, an open thread), beta (one decision, folds), gamma (two decisions),
// learnings linked and tagged, and raw inbox notes.
func topicVault(t *testing.T) http.Handler {
	t.Helper()
	v := &core.Vault{Root: t.TempDir()}
	dec := func(id, status, upd, desc string, tags ...any) {
		writeArtifact(t, v, core.TypeDecision, id, map[string]any{"title": "T " + id, "status": status, "updated": upd, "description": desc, "tags": tags}, "x\n")
	}
	dec("alpha.0001-first", "accepted", "2026-10-01", "Old alpha line.", "domain/ui")
	dec("alpha.0002-second", "accepted", "2026-10-05", "Newest alpha line.")
	dec("alpha.0003-third", "superseded", "2026-10-09", "Superseded line.")
	dec("beta.0001-only", "proposed", "2026-10-08", "Beta line.")
	dec("gamma.0001-a", "accepted", "2026-10-03", "Gamma a.")
	dec("gamma.0002-b", "rejected", "2026-10-04", "Gamma b.")
	writeArtifact(t, v, core.TypeThread, "alpha.0001-question", map[string]any{"title": "Is it so", "status": "open", "updated": "2026-10-02"}, "x\n")
	writeArtifact(t, v, core.TypeLearning, "linked-one", map[string]any{"title": "Linked learning", "related": []any{"[[decision.alpha.0001-first]]"}, "updated": "2026-10-06"}, "x\n")
	writeArtifact(t, v, core.TypeLearning, "tagged-one", map[string]any{"title": "Tagged learning", "tags": []any{"domain/ui"}, "updated": "2026-10-07"}, "x\n")
	writeArtifact(t, v, core.TypeLearning, "other-one", map[string]any{"title": "Other learning", "tags": []any{"domain/db"}}, "x\n")
	writeArtifact(t, v, core.TypeInbox, "2026-10-09-routed", map[string]any{"title": "Routed note", "status": "raw", "updated": "2026-10-09"}, "Body.\n\n## Route\n\nA later milestone.\n\nSecond line.\n")
	writeArtifact(t, v, core.TypeInbox, "2026-10-08-plain", map[string]any{"title": "Plain note", "status": "raw", "updated": "2026-10-08"}, "Mentions the route of a thing only.\n")
	writeArtifact(t, v, core.TypeInbox, "2026-10-07-cites", map[string]any{"title": "Cites alpha", "status": "raw", "updated": "2026-10-07"}, "The alpha topic matters.\n")
	writeArtifact(t, v, core.TypeInbox, "2026-10-06-linked", map[string]any{"title": "Links by related", "status": "raw", "updated": "2026-10-06", "related": []any{"[[decision.alpha.0002-second]]"}}, "No topic name here.\n")
	return serve(t, v)
}

// Warrant: fails if the home page stops being the knowledge page, keeps the retired tree or Now band, or loses a count link.
func TestKnowledge_ReplacesHome(t *testing.T) {
	code, body := do(topicVault(t), "GET", "/")
	if code != 200 {
		t.Fatalf("GET / = %d", code)
	}
	for _, want := range []string{
		`<div class="knowledge">`, `<h1>Knowledge</h1>`, `href="/type/decision">6 decisions`, `href="/type/learning">3 learnings`,
		`href="/type/thread">1 thread<`, `href="/type/inbox?status=raw">4 raw inbox notes`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("knowledge page lacks %q", want)
		}
	}
	for _, bad := range []string{`class="tree"`, `class="now"`, "Design spine"} {
		if strings.Contains(body, bad) {
			t.Errorf("knowledge page still has %q", bad)
		}
	}
}

// Warrant: fails if a one-decision topic becomes a row, a row loses its link, sort or composed line, or the superseded decision supplies the line.
func TestKnowledge_TopicRows(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/")
	list, _, _ := strings.Cut(body, `<details class="other">`)
	inOrder(t, list, `href="/topic/alpha"`, "Newest alpha line; open: Is it so", "2 accepted", "1 superseded", "1 open", "thread", `href="/topic/gamma"`, "Gamma a")
	if strings.Contains(list, "Superseded line.") || strings.Contains(list, `href="/topic/beta"`) {
		t.Errorf("rows hold a superseded line or a folded topic:\n%s", list)
	}
	inOrder(t, body, `<details class="other">`, "Other topics, 1", `href="/topic/beta"`, "proposed")
}

// Warrant: fails if a note without a Route heading is listed, the Route line is not the section's first line, or a non-raw note appears.
func TestKnowledge_RoutedInbox(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/")
	_, inbox, _ := strings.Cut(body, `id="inbox"`)
	inOrder(t, inbox, "Routed note", "Route: A later milestone.")
	for _, bad := range []string{"Plain note", "Second line.", "Cites alpha"} {
		if strings.Contains(inbox, bad) {
			t.Errorf("routed inbox lists %q", bad)
		}
	}
}

// newTagVault seeds topic alpha tagged domain/ui and n learnings carrying the tag.
func newTagVault(t *testing.T, n int) *core.Vault {
	t.Helper()
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeDecision, "alpha.0001-first", map[string]any{"title": "First", "status": "accepted", "tags": []any{"domain/ui"}}, "x\n")
	for i := 0; i < n; i++ {
		writeArtifact(t, v, core.TypeLearning, fmt.Sprintf("l%02d", i), map[string]any{"title": fmt.Sprintf("L %02d", i), "tags": []any{"domain/ui"}}, "x\n")
	}
	return v
}

// Warrant: fails if the Routed lede loses the raw-note total or its singular verb, an Other-only vault renders the column head over an empty list, the one-folded-topic lede loses its singular verb and "It holds" wording or keeps "below,", or an empty vault prints a lede of zeros.
func TestKnowledge_LedeAndOtherOnly(t *testing.T) {
	_, body := do(topicVault(t), "GET", "/")
	if !strings.Contains(body, "1 of 4 raw notes carries a route, newest first.") {
		t.Errorf("routed lede wrong:\n%s", body)
	}
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeDecision, "lone.0001-a", map[string]any{"title": "Lone", "status": "accepted", "updated": "2026-10-01", "description": "x"}, "x\n")
	_, body = do(serve(t, v), "GET", "/")
	if strings.Contains(body, `class="topic-head"`) || strings.Contains(body, "No decision or thread carries a topic yet.") || !strings.Contains(body, `class="other"`) {
		t.Errorf("other-only page wrong:\n%s", body)
	}
	if strings.Contains(body, " below,") || !strings.Contains(body, "1 topic, read from the ids of 1 decision and 0 threads; 0 decisions are proposed and 0 threads open. It holds one decision and folds below. Learnings") {
		t.Errorf("no-listed lede wrong:\n%s", body)
	}
	_, body = do(serve(t, &core.Vault{Root: t.TempDir()}), "GET", "/")
	if !strings.Contains(body, "No decision or thread carries a topic yet.") {
		t.Error("empty vault lacks the none-yet line")
	}
}
