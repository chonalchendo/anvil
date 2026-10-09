package ui

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/chonalchendo/anvil/internal/core"
)

// diagramCSP keeps a diagram inert: inline styles, own-origin fonts and data
// images only. It is the second wall behind the no-<script> refusal.
const diagramCSP = "default-src 'none'; style-src 'unsafe-inline'; font-src 'self'; img-src data:"

// canvas is one diagram on a pan-and-zoom surface; Full fills the viewport.
type canvas struct {
	Name string
	Note string
	Full bool
	Lazy bool
}

// diagramsOf lists the names in a design's diagrams slot, in slot order.
func diagramsOf(fm map[string]any) []canvas {
	var out []canvas
	for _, name := range core.DiagramNames(fm) {
		out = append(out, canvas{Name: name})
	}
	return out
}

// diagramFile reads the named diagram and returns it with the cleaned name. It
// writes the refusal and returns false when the file is missing (404),
// unreadable (500) or holds a script (415). filepath.Base is the only path
// defence: the name never leaves _meta/diagrams.
func (s *server) diagramFile(w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	name := filepath.Base(r.PathValue("name"))
	b, err := os.ReadFile(core.DiagramPath(s.v.Root, name))
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return "", nil, false
	}
	if err != nil {
		slog.Error("reading diagram", "err", err)
		http.Error(w, "diagram unreadable", http.StatusInternalServerError)
		return "", nil, false
	}
	if bytes.Contains(bytes.ToLower(b), []byte("<script")) {
		http.Error(w, "diagram holds a script", http.StatusUnsupportedMediaType)
		return "", nil, false
	}
	return name, b, true
}

// diagram renders the full-page canvas for one diagram.
func (s *server) diagram(w http.ResponseWriter, r *http.Request) {
	name, _, ok := s.diagramFile(w, r)
	if !ok {
		return
	}
	s.render(w, r, "diagram", struct {
		Title  string
		Canvas canvas
	}{name, canvas{Name: name, Full: true}})
}

// diagramSrc serves the diagram file itself, for the canvas frame.
func (s *server) diagramSrc(w http.ResponseWriter, r *http.Request) {
	_, b, ok := s.diagramFile(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Security-Policy", diagramCSP)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
