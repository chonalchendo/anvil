package ui

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

//go:embed templates
var templateFS embed.FS

// pages holds one parsed template set per page: base plus that page's content.
type pages map[string]*template.Template

func loadPages(a assets) (pages, error) {
	funcs := template.FuncMap{"asset": a.url}
	out := pages{}
	files, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		name := strings.TrimSuffix(path.Base(f), ".html")
		if name == "base" {
			continue
		}
		t, err := template.New("base.html").Funcs(funcs).ParseFS(templateFS, "templates/base.html", f)
		if err != nil {
			return nil, fmt.Errorf("parsing %s template: %w", name, err)
		}
		out[name] = t
	}
	return out, nil
}

// render buffers the page so a template error yields a clean 500, not a
// half-written 200.
func (p pages) render(w http.ResponseWriter, name string, sb sidebar, data any) {
	var buf bytes.Buffer
	if err := p[name].Execute(&buf, struct {
		Sidebar sidebar
		Page    any
	}{sb, data}); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// render builds the sidebar once per request, then renders the page.
func (s *server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	sb, err := s.sidebar(r)
	if err != nil {
		slog.Error("building sidebar", "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.pages.render(w, name, sb, data)
}
