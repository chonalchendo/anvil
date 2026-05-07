package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListOrphansReturnsArtifactsWithNoIncomingLinks(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	// Lonely: no incoming or outgoing links → orphan.
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "lonely",
		"--description", "lonely desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	// Popular: target of a link → NOT orphan.
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "popular",
		"--description", "popular desc",
		"--tags", "domain/dev-tools",
	)
	// Linker: source of a link → orphan (no incoming).
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "linker",
		"--description", "linker desc",
		"--tags", "domain/dev-tools",
	)
	execCmd(t, "link", "issue", "demo.linker", "issue", "demo.popular")

	out := execCmd(t, "list", "issue", "--orphans", "--json")
	var env struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	got := map[string]bool{}
	for _, it := range env.Items {
		got[it.ID] = true
	}
	if !got["demo.lonely"] {
		t.Fatalf("lonely missing from orphans: %v", env.Items)
	}
	if got["demo.popular"] {
		t.Fatalf("popular should NOT be in orphans: %v", env.Items)
	}
}
