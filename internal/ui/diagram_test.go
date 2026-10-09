package ui

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeDiagram(t *testing.T, v *core.Vault, name, html string) {
	t.Helper()
	p := core.DiagramPath(v.Root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDiagramPage_HoldsSandboxedCanvas(t *testing.T) {
	h, _ := seed(t)
	code, body := do(h, "GET", "/diagram/anvil-two-loop")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	for _, want := range []string{`class="canvas"`, `<iframe sandbox="allow-same-origin" src="/diagram-src/anvil-two-loop"`, `data-full`, `<kbd>f</kbd> fit`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if strings.Contains(body, `href="/diagram/anvil-two-loop"`) {
		t.Error("full page links to itself")
	}
}

func TestDiagramSrc_ServesWithCSP(t *testing.T) {
	h, _ := seed(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/diagram-src/anvil-two-loop", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<svg>") {
		t.Fatalf("status = %d body %q", rec.Code, rec.Body.String())
	}
	want := "default-src 'none'; style-src 'unsafe-inline'; font-src 'self'; img-src data:"
	if got := rec.Header().Get("Content-Security-Policy"); got != want {
		t.Errorf("CSP = %q, want %q", got, want)
	}
}

func TestDiagramRoutes_Refusals(t *testing.T) {
	h, v := seed(t)
	writeDiagram(t, v, "scripted", "<html><SCRIPT>alert(1)</SCRIPT></html>")
	for _, p := range []string{filepath.Join(v.Root, "secret.html"), filepath.Join(v.Root, "_meta", "secret.html")} {
		if err := os.WriteFile(p, []byte("<p>secret</p>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		path string
		want int
	}{
		{"/diagram/scripted", 415},
		{"/diagram-src/scripted", 415},
		{"/diagram/missing", 404},
		{"/diagram-src/missing", 404},
		{"/diagram-src/..%2Fsecret", 404},
		{"/diagram-src/..%2F..%2Fsecret", 404},
	} {
		if code, _ := do(h, "GET", c.path); code != c.want {
			t.Errorf("GET %s = %d, want %d", c.path, code, c.want)
		}
	}
}

func TestDiagramCanvas_OnDesignPageOnly(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/product-design.anvil")
	for _, want := range []string{`class="canvas"`, `href="/diagram/anvil-two-loop"`, `<kbd>f</kbd> fit`, `<kbd>0</kbd> 1:1`} {
		if !strings.Contains(body, want) {
			t.Errorf("design page lacks %s", want)
		}
	}
	if strings.Index(body, `class="canvas"`) < strings.Index(body, `<header class="node">`) || strings.Index(body, `class="canvas"`) > strings.Index(body, `<section class="body">`) {
		t.Error("canvas is not between the header and the body")
	}
	_, body = do(h, "GET", decisionPath)
	for _, dead := range []string{`class="canvas"`, `<kbd>f</kbd>`, `<kbd>0</kbd>`} {
		if strings.Contains(body, dead) {
			t.Errorf("decision page holds %s", dead)
		}
	}
}
