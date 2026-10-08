package ui

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
)

//go:embed templates
var templateFS embed.FS

// sidebarTypes are the type lists the sidebar links; 0351 serves the routes.
var sidebarTypes = []string{"convention", "decision", "learning", "thread", "inbox"}

// pages holds one parsed template set per page: base plus that page's content.
type pages map[string]*template.Template

func loadPages(a assets) (pages, error) {
	funcs := template.FuncMap{"asset": a.url}
	out := pages{}
	for _, name := range []string{"home", "artifact"} {
		t, err := template.New("base.html").Funcs(funcs).ParseFS(templateFS, "templates/base.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("parsing %s template: %w", name, err)
		}
		out[name] = t
	}
	return out, nil
}

// render buffers the page so a template error yields a clean 500, not a
// half-written 200.
func (p pages) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := p[name].Execute(&buf, struct {
		Types []string
		Page  any
	}{sidebarTypes, data}); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
