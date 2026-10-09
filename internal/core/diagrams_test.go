package core

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/chonalchendo/anvil/internal/schema"
)

// Warrant: a schema-valid name the Go pattern rejects makes validate skip the
// file check, so the two patterns must stay equal.
func TestDiagramNamePatternMatchesSchemas(t *testing.T) {
	for _, typ := range []string{"product-design", "system-design", "component-design"} {
		raw, err := schema.EmbeddedFS.ReadFile(typ + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		var s struct {
			Properties struct {
				Diagrams struct {
					Items struct {
						Pattern string `json:"pattern"`
					} `json:"items"`
				} `json:"diagrams"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		if got, want := s.Properties.Diagrams.Items.Pattern, diagramName.String(); got != want {
			t.Errorf("%s: schema pattern %q != Go pattern %q", typ, got, want)
		}
	}
}

func TestDiagramNames_SkipsNonStringItems(t *testing.T) {
	got := DiagramNames(map[string]any{"diagrams": []any{"a-b", 7, "c"}})
	if want := []string{"a-b", "c"}; !slices.Equal(got, want) {
		t.Errorf("DiagramNames = %v, want %v", got, want)
	}
}
