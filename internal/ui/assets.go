package ui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"time"
)

//go:embed static
var staticFS embed.FS

// assets serves the embedded static files. Each URL carries a content hash, so
// a changed file gets a new URL and the response can be cached forever.
type assets struct {
	files map[string][]byte
	ver   map[string]string
}

func loadAssets() (assets, error) {
	a := assets{files: map[string][]byte{}, ver: map[string]string{}}
	ents, err := fs.ReadDir(staticFS, "static")
	if err != nil {
		return a, err
	}
	for _, e := range ents {
		b, err := staticFS.ReadFile("static/" + e.Name())
		if err != nil {
			return a, err
		}
		sum := sha256.Sum256(b)
		a.files[e.Name()] = b
		a.ver[e.Name()] = hex.EncodeToString(sum[:])[:8]
	}
	return a, nil
}

// url is the template-facing `asset` func.
func (a assets) url(name string) string {
	return "/static/" + name + "?v=" + a.ver[name]
}

func (a assets) serve(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	b, ok := a.files[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
}
