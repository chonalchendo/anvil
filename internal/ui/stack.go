package ui

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/hydrate"
)

// layer is one hydrate node. Order, Size and Body mirror what the walk loads.
type layer struct {
	Type, Status, Glyph, Title, Size string
	Ref                              link
	Body                             template.HTML
}

type stackPage struct {
	Title, Key string
	Total      string
	Layers     []layer
	Broken     []hydrate.BrokenEdge
	Skipped    []string
}

func (s *server) stack(w http.ResponseWriter, r *http.Request) {
	arg := r.PathValue("id")
	if arg == "" || strings.ContainsAny(arg, `/\`) {
		http.NotFound(w, r)
		return
	}
	id, err := core.ResolveIssueArg(s.v, arg)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h, err := hydrate.Assemble(s.v, id)
	var missing *hydrate.NotFoundError
	if errors.As(err, &missing) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("assembling stack", "issue", id, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	page, err := s.buildStack(core.IndexKey(core.TypeIssue, id), h)
	if err != nil {
		slog.Error("building stack page", "issue", id, "err", err)
		http.Error(w, "page failed", http.StatusInternalServerError)
		return
	}
	s.pages.render(w, "stack", page)
}

func (s *server) buildStack(key string, h *hydrate.Hydration) (stackPage, error) {
	page := stackPage{Title: key, Key: key, Broken: h.Broken, Skipped: h.SkippedBodyLinks}
	total := 0
	for _, n := range h.Nodes {
		body, err := s.md.render(n.Body)
		if err != nil {
			return stackPage{}, err
		}
		nk := core.IndexKey(n.Type, n.ID)
		title, _ := n.FrontMatter["title"].(string)
		if title == "" {
			title = nk
		}
		total += len(n.Body)
		page.Layers = append(page.Layers, layer{
			Type: string(n.Type), Status: n.Status, Glyph: glyphs[n.Status], Title: title,
			Size: kb(len(n.Body)), Ref: link{Text: nk, Href: artifactHref(nk)}, Body: body,
		})
	}
	if len(page.Layers) > 0 {
		page.Title = page.Layers[0].Title
	}
	page.Total = kb(total)
	return page, nil
}

func kb(n int) string { return fmt.Sprintf("%.1f KB", float64(n)/1024) }
