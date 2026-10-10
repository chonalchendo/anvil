package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func writeArtifact(t *testing.T, v *core.Vault, typ core.Type, id string, fm map[string]any, body string) {
	t.Helper()
	fm["type"] = string(typ)
	path := filepath.Join(v.Root, typ.Dir(), id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// seed builds a vault with one artifact of each prefix-less type, a dangling
// link, a script body, a milestone chain for the breadcrumb and an issue whose
// hydrated stack has distinct layer sizes and a broken edge.
func seed(t *testing.T) (http.Handler, *core.Vault) {
	t.Helper()
	v := &core.Vault{Root: t.TempDir()}
	writeArtifact(t, v, core.TypeProductDesign, "anvil", map[string]any{"title": "Anvil product", "diagrams": []any{"anvil-two-loop"}}, "## TL;DR\n\nproduct\n")
	writeArtifact(t, v, core.TypeThread, "anvil-design-docs.0002-x", map[string]any{"title": "A thread"}, "thread\n")
	writeArtifact(t, v, core.TypeLearning, "a-learning", map[string]any{"title": "A learning"}, "learning\n"+strings.Repeat("x", 4096))
	writeArtifact(t, v, core.TypeIssue, stackIssue, map[string]any{
		"title": "Thing", "status": "in-progress",
		"project": "anvil", "updated": "2026-10-09", "description": "Deck line",
		"milestone": "[[milestone.anvil.m1]]",
		"learnings": []any{"[[learning.a-learning]]", "[[learning.ghost]]"},
	}, strings.Repeat("x", 2048)+"\n\n## Links\n\n- [[thread.anvil-design-docs.0002-x]]\n")
	writeArtifact(t, v, core.TypeMilestone, "milestone.anvil.m1", map[string]any{
		"title": "M1", "product_design": "[[product-design.anvil]]",
	}, "milestone body\n")
	writeArtifact(t, v, core.TypeDecision, "ui.0001-a-decision", map[string]any{
		"title":   "A decision",
		"related": []any{"[[product-design.anvil]]", "[[milestone.anvil.m1]]"},
	}, "See [[product-design.anvil]], [[thread.anvil-design-docs.0002-x]], [[learning.a-learning|the learning]] and [[thread.gone.0001-missing]].\n\n"+
		"Code `[[product-design.anvil]]` stays literal.\n\n```\n[[product-design.anvil]]\n```\n\n<script>alert(1)</script>\n\nPath is /artifact/<key> here.\n\nRun `anvil transition issue x resolved`.\n")
	writeDiagram(t, v, "anvil-two-loop", "<html><body><svg></svg></body></html>")
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
	return h, v
}

func do(h http.Handler, method, path string) (int, string) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	b, _ := io.ReadAll(rec.Body)
	return rec.Code, string(b)
}

const (
	decisionPath = "/artifact/decision.ui.0001-a-decision"
	stackPath    = "/issue/" + stackIssue + "/stack"
)

func TestArtifactPage_ResolvesPrefixlessLink(t *testing.T) {
	h, _ := seed(t)
	code, body := do(h, "GET", decisionPath)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	for _, want := range []string{
		`<a href="/artifact/product-design.anvil">`,
		`<a href="/artifact/thread.anvil-design-docs.0002-x">`,
		`<a href="/artifact/learning.a-learning">the learning</a>`,
		`<a href="/artifact/milestone.anvil.m1">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
}

func TestArtifactPage_UnresolvedLinkSpan(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", decisionPath)
	if !strings.Contains(body, `<span class="unresolved">thread.gone.0001-missing</span>`) {
		t.Error("dangling link is not an unresolved span")
	}
	if strings.Contains(body, `href="/artifact/thread.gone`) {
		t.Error("dangling link rendered as an anchor")
	}
}

func TestArtifactPage_CodeLinksStayLiteral(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", decisionPath)
	if got := strings.Count(body, "[[product-design.anvil]]"); got != 2 {
		t.Errorf("literal wikilinks = %d, want 2 (code span + fence)", got)
	}
}

func TestArtifactPage_RawHTMLEscapedCommandIsCode(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", decisionPath)
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("body <script> rendered raw")
	}
	for _, want := range []string{"&lt;script&gt;", "&lt;key&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("raw HTML %s not escaped into visible text", want)
		}
	}
	if !strings.Contains(body, "<code>anvil transition issue x resolved</code>") {
		t.Error("command is not plain code text")
	}
}

func TestArtifactPage_CitedByAndBreadcrumb(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/artifact/product-design.anvil")
	if !strings.Contains(body, `<details class="cited">`) || !strings.Contains(body, `href="/artifact/decision.ui.0001-a-decision"`) {
		t.Error("incoming link from the decision missing")
	}
	_, body = do(h, "GET", "/artifact/milestone.anvil.m1")
	if !strings.Contains(body, `class="crumbs"`) || !strings.Contains(body, `href="/artifact/product-design.anvil"`) {
		t.Error("breadcrumb to the product design missing")
	}
}

func TestArtifactPage_NotFound(t *testing.T) {
	h, _ := seed(t)
	for _, p := range []string{"/artifact/bogus.x", "/artifact/thread.nope", "/artifact/thread.", "/nope", "/artifact/", "/static/nope.css"} {
		if code, _ := do(h, "GET", p); code != 404 {
			t.Errorf("GET %s = %d, want 404", p, code)
		}
	}
}

func TestRoutes_PostReturns405(t *testing.T) {
	h, _ := seed(t)
	for _, p := range []string{"/", decisionPath, stackPath, "/type/decision", "/palette", "/search?q=x", "/compare?a=product-design.anvil&b=decision.ui.0001-a-decision", "/diagram/anvil-two-loop", "/diagram-src/anvil-two-loop", "/static/anvil.css"} {
		for _, m := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			if code, _ := do(h, m, p); code != 405 {
				t.Errorf("%s %s = %d, want 405", m, p, code)
			}
		}
		if code, _ := do(h, "HEAD", p); code != 200 {
			t.Errorf("HEAD %s = %d, want 200", p, code)
		}
	}
}

func TestHome_DarkShell(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/")
	for _, want := range []string{`<html lang="en" style="color-scheme: dark">`, `href="/type/convention"`, `href="/type/inbox"`, `href="/"`} {
		if !strings.Contains(body, want) {
			t.Errorf("home lacks %s", want)
		}
	}
}

func TestStatic_VersionedAndImmutable(t *testing.T) {
	h, _ := seed(t)
	_, body := do(h, "GET", "/")
	_, rewritten := do(h, "GET", "/static/anvil.css")
	sum := sha256.Sum256([]byte(rewritten))
	want := `/static/anvil.css?v=` + hex.EncodeToString(sum[:])[:8]
	if !strings.Contains(body, want) {
		t.Errorf("home lacks %s", want)
	}
	font, _ := staticFS.ReadFile("static/inter.woff2")
	fsum := sha256.Sum256(font)
	if want := `url("inter.woff2?v=` + hex.EncodeToString(fsum[:])[:8] + `")`; !strings.Contains(rewritten, want) {
		t.Errorf("served css lacks versioned font %s", want)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/static/inter.woff2?v=x", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("font response = %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	for _, f := range []string{"OFL-Inter.txt", "OFL-GeistMono.txt", "geist-mono.woff2", "anvil.css"} {
		if code, _ := do(h, "GET", "/static/"+f); code != 200 {
			t.Errorf("static %s = %d", f, code)
		}
	}
}

func TestServe_RefusesNonLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", "192.168.1.5:7780", ":7780", "example.com:80", "nonsense"} {
		err := Serve(context.Background(), nil, nil, addr, io.Discard)
		var nl *ErrAddrNotLoopback
		if !errors.As(err, &nl) || nl.Addr != addr {
			t.Errorf("Serve(%q) err = %v, want ErrAddrNotLoopback", addr, err)
		}
	}
	for _, addr := range []string{"127.0.0.1:0", "localhost:0", "[::1]:0", "127.0.0.2:0"} {
		if err := requireLoopback(addr); err != nil {
			t.Errorf("requireLoopback(%q) = %v", addr, err)
		}
	}
}

func TestRequestsDoNotWriteVault(t *testing.T) {
	h, v := seed(t)
	before := hashTree(t, v.Root)
	for _, p := range []string{"/", decisionPath, stackPath, "/artifact/milestone.anvil.m1", "/type/decision", "/palette", "/search?q=x", "/compare?a=product-design.anvil&b=decision.ui.0001-a-decision", "/diagram/anvil-two-loop", "/diagram-src/anvil-two-loop", "/static/anvil.css"} {
		do(h, "GET", p)
		do(h, "POST", p)
	}
	if after := hashTree(t, v.Root); after != before {
		t.Error("vault tree changed after requests")
	}
}

func hashTree(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, ".anvil") {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // path comes from walking a t.TempDir()
		h.Write([]byte(p))
		h.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(h.Sum(nil))
}

func TestTemplates_RenderOnFixtureData(t *testing.T) {
	a, err := loadAssets()
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadPages(a)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.render(rec, "artifact", sidebar{}, artifactPage{
		Title: "T", Key: "k.x", Crumbs: []link{{Text: "c", Href: "/artifact/c"}},
		Props: []prop{{Name: "n", Values: []link{{Text: "v", Plain: true}}}},
		Cited: []citedGroup{{Type: "s", Count: 1, Items: []link{{Text: "x"}}}}, CitedTotal: 1,
	})
	if rec.Code != 200 {
		t.Fatalf("artifact template status = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	p.render(rec, "knowledge", sidebar{}, knowledgePage{})
	if rec.Code != 200 {
		t.Fatalf("knowledge template status = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	p.render(rec, "topic", sidebar{}, topicPage{})
	if rec.Code != 200 {
		t.Fatalf("topic template status = %d", rec.Code)
	}
}

func TestPackage_DoesNotImportCLI(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.Contains(dep, "/internal/cli") {
			t.Errorf("internal/ui depends on %s", dep)
		}
	}
}
