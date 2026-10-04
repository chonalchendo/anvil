package anvil

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// The guide loads into every session, so a renamed type must fail here, not
// mislead agents (contract → component-design shipped a stale guide).
func TestSessionGuideNamesOnlyRealTypes(t *testing.T) {
	const marker = "Types include "
	_, rest, ok := strings.Cut(SessionGuide, marker)
	if !ok {
		t.Fatalf("guide has no %q line", marker)
	}
	line, _, _ := strings.Cut(rest, "\n")
	for _, name := range strings.Split(strings.TrimSuffix(line, "."), ", ") {
		if _, err := core.ParseType(name); err != nil {
			t.Errorf("guide names unknown type %q: %v", name, err)
		}
	}
}
