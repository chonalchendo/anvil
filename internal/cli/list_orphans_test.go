package cli

import (
	"strings"
	"testing"
)

func TestListOrphansReturnsArtifactsWithNoIncomingLinks(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	// Use legacy-format fixture files for stable IDs.
	writeFixtureIssueDated(t, vault, "demo", "lonely", "lonely", "2026-01-01")
	writeFixtureIssueDated(t, vault, "demo", "popular", "popular", "2026-01-02")
	writeFixtureIssueDated(t, vault, "demo", "linker", "linker", "2026-01-03")
	execCmd(t, "reindex")
	execCmd(t, "link", "issue", "demo.linker", "issue", "demo.popular")

	out := execCmdJSON(t, "list", "issue", "--orphans", "--json")
	var env struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := jsonUnmarshal(t, strings.TrimSpace(out), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	got := map[string]bool{}
	for _, it := range env.Items {
		got[it.ID] = true
	}
	if !got["issue.demo.lonely"] {
		t.Errorf("lonely missing from orphans: %v", env.Items)
	}
	if !got["issue.demo.linker"] {
		t.Errorf("linker missing from orphans (source-only edges don't count as incoming): %v", env.Items)
	}
	if got["issue.demo.popular"] {
		t.Errorf("popular should NOT be in orphans: %v", env.Items)
	}
}
