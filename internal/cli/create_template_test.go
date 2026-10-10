package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestTemplateSectionsForTypeDesigns(t *testing.T) {
	cases := map[core.Type][]string{
		core.TypeProductDesign: core.RequiredProductDesignSections,
		core.TypeSystemDesign:  core.RequiredSystemDesignSections,
	}
	for typ, want := range cases {
		if diff := cmp.Diff(want, templateSectionsForType(typ)); diff != "" {
			t.Errorf("templateSectionsForType(%s) mismatch (-want +got):\n%s", typ, diff)
		}
	}
}
