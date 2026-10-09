package ui

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// paletteEntry is the shape the ⌘K script in base.html reads.
type paletteEntry struct {
	Key    string `json:"key"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// palette serves one entry per indexed artifact. Failures return a non-OK
// status so the client does not cache a partial list.
func (s *server) palette(w http.ResponseWriter, _ *http.Request) {
	out := []paletteEntry{}
	for _, t := range core.AllTypes {
		rows, err := s.db.ListByType(string(t), index.QueryFilters{})
		if err != nil {
			slog.Error("building palette", "type", t, "err", err)
			http.Error(w, "palette failed", http.StatusInternalServerError)
			return
		}
		for _, r := range rows {
			title := r.Title
			if title == "" {
				title = r.ID
			}
			out = append(out, paletteEntry{Key: r.ID, Type: r.Type, Title: title, Status: r.Status})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
