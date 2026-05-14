package schema_test

import (
	"testing"

	"github.com/chonalchendo/anvil/internal/schema"
)

// TestExternalLinksAcceptedOnAllTypes asserts the universal external_links
// affordance is declared on every artifact schema. The field is a string array
// so `anvil link --external` can append commit shas / PR urls / generic URIs
// without per-kind schema expansion (issue acceptance #4).
func TestExternalLinksAcceptedOnAllTypes(t *testing.T) {
	types := []string{
		"decision", "inbox", "issue", "learning", "milestone",
		"plan", "product-design", "session", "sweep",
		"system-design", "thread",
	}
	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			kind, err := schema.FieldKind(typ, "external_links")
			if err != nil {
				t.Fatalf("FieldKind(%q, external_links): %v", typ, err)
			}
			if kind != schema.KindArray {
				t.Fatalf("external_links on %s: kind = %v, want KindArray", typ, kind)
			}
		})
	}
}
