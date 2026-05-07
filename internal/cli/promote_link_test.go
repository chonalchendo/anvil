package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPromoteWritesWikilinkSoLinkQueryFindsInbox confirms the inbox's
// `promoted_to` field is stored as a wikilink and so `link --to <issue-id>`
// surfaces the originating inbox row. AGENTS.md tells agents to check this
// before promoting another inbox entry covering the same work; if `promote`
// stored the bare id, the index would skip the edge and the check would be
// silently empty.
func TestPromoteWritesWikilinkSoLinkQueryFindsInbox(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	out := execCmd(t, "create", "inbox", "--title", "fix-the-thing", "--description", "drive-by", "--json")
	var created struct{ ID string }
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &created); err != nil {
		t.Fatalf("create json: %v\nout: %s", err, out)
	}

	promote := execCmd(t, "promote", created.ID,
		"--as", "issue",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json")
	var p struct {
		TargetID *string `json:"target_id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(promote)), &p); err != nil {
		t.Fatalf("promote json: %v\nout: %s", err, promote)
	}
	if p.TargetID == nil || *p.TargetID == "" {
		t.Fatalf("promote returned no target_id: %s", promote)
	}

	// `link --to <issue-id>` must find the inbox edge with relation=promoted_to.
	queryOut := execCmd(t, "link", "--to", *p.TargetID, "--json")
	var rows []struct {
		Source, Target, Relation string
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(queryOut)), &rows); err != nil {
		t.Fatalf("link --to json: %v\nout: %s", err, queryOut)
	}
	var found bool
	for _, r := range rows {
		if r.Source == created.ID && r.Relation == "promoted_to" && r.Target == *p.TargetID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected promoted_to edge from %s -> %s in link --to result, got: %v",
			created.ID, *p.TargetID, rows)
	}
}
