package ui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
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
		a.files[e.Name()] = b
	}
	// Fonts hash first: the CSS names them by versioned URL, so the CSS hash
	// covers the font versions too.
	for name, b := range a.files {
		if path.Ext(name) == ".woff2" {
			a.ver[name] = hash8(b)
		}
	}
	css := a.files["anvil.css"]
	for name := range a.ver {
		css = bytes.ReplaceAll(css, []byte(`url("`+name+`")`), []byte(`url("`+name+`?v=`+a.ver[name]+`")`))
	}
	a.files["anvil.css"] = css
	for name, b := range a.files {
		if _, done := a.ver[name]; !done {
			a.ver[name] = hash8(b)
		}
	}
	return a, nil
}

func hash8(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:8]
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
