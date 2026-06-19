package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// writeReadyIssueFile writes an open issue with a custom severity/created and an
// optional milestone + contract links, returning the index row that points at
// it. Used to exercise selectReadyUnits' enrichment and deterministic sort
// without standing up the index.
func writeReadyIssueFile(t *testing.T, vault, id, severity, created, milestone string, related []any) index.ArtifactRow {
	t.Helper()
	path := filepath.Join(vault, "70-issues", id+".md")
	fm := map[string]any{
		"type": "issue", "title": id, "description": "fixture description",
		"created": created, "updated": created,
		"status": "open", "project": "demo", "severity": severity,
		"tags": []any{"domain/dev-tools"}, "goal": "goal of " + id,
	}
	if milestone != "" {
		fm["milestone"] = "[[milestone." + milestone + "]]"
	}
	if related != nil {
		fm["related"] = related
	}
	a := &core.Artifact{Path: path, FrontMatter: fm, Body: fixtureIssueBody}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return index.ArtifactRow{ID: id, Type: "issue", Status: "open", Project: "demo", Path: path, Created: created}
}

func TestSelectReadyUnits_OrdersBySeverityThenCreatedThenID(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	// Deliberately out of priority order on input; the row slice mimics
	// ListReady's id-asc output so the sort is doing the real work.
	rows := []index.ArtifactRow{
		writeReadyIssueFile(t, vault, "demo.a-low", "low", "2026-01-01", "", nil),
		writeReadyIssueFile(t, vault, "demo.b-crit-late", "critical", "2026-02-01", "", nil),
		writeReadyIssueFile(t, vault, "demo.c-crit-early", "critical", "2026-01-01", "", nil),
		writeReadyIssueFile(t, vault, "demo.d-high", "high", "2026-03-01", "", nil),
		writeReadyIssueFile(t, vault, "demo.e-crit-early-z", "critical", "2026-01-01", "", nil),
	}

	units := selectReadyUnits(rows, "")
	got := make([]string, len(units))
	for i, u := range units {
		got[i] = u.ID
	}
	// critical (created asc, then id asc) → high → low.
	want := []string{"demo.c-crit-early", "demo.e-crit-early-z", "demo.b-crit-late", "demo.d-high", "demo.a-low"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSelectReadyUnits_EnrichesGoalAndContracts(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	row := writeReadyIssueFile(t, vault, "demo.x", "high", "2026-01-01", "",
		[]any{"[[contract.demo.c1]]", "[[issue.demo.sibling]]", "[[contract.demo.c2]]"})
	units := selectReadyUnits([]index.ArtifactRow{row}, "")
	if len(units) != 1 {
		t.Fatalf("got %d units, want 1", len(units))
	}
	u := units[0]
	if u.Goal != "goal of demo.x" {
		t.Errorf("goal = %q, want %q", u.Goal, "goal of demo.x")
	}
	// Only contract relations survive; the sibling issue link is dropped.
	if strings.Join(u.Contracts, ",") != "demo.c1,demo.c2" {
		t.Errorf("contracts = %v, want [demo.c1 demo.c2]", u.Contracts)
	}
}

func TestSelectReadyUnits_MilestoneFilter(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	rows := []index.ArtifactRow{
		writeReadyIssueFile(t, vault, "demo.m", "high", "2026-01-01", "demo.m1", nil),
		writeReadyIssueFile(t, vault, "demo.n", "high", "2026-01-01", "", nil),
	}
	units := selectReadyUnits(rows, "demo.m1")
	if len(units) != 1 || units[0].ID != "demo.m" {
		t.Fatalf("milestone filter: got %v, want [demo.m]", units)
	}
}

// TestNext_JSON_ReturnsHeadDeterministically is the Direct cobra-root smoke for
// `anvil next`: through the real command tree it returns the single
// highest-priority ready unit, with start-context fields, identically across
// repeated calls.
func TestNext_JSON_ReturnsHeadDeterministically(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeReadyIssueFile(t, vault, "demo.low", "low", "2026-01-01", "", nil)
	writeReadyIssueFile(t, vault, "demo.crit", "critical", "2026-02-01", "", []any{"[[contract.demo.c1]]"})
	execCmd(t, "reindex")

	out1 := execCmd(t, "next", "--json", "--project", "demo")
	out2 := execCmd(t, "next", "--json", "--project", "demo")
	if strings.TrimSpace(out1) != strings.TrimSpace(out2) {
		t.Errorf("next --json not deterministic:\nfirst:\n%s\nsecond:\n%s", out1, out2)
	}

	var u struct {
		ID        string   `json:"id"`
		Goal      string   `json:"goal"`
		Severity  string   `json:"severity"`
		Contracts []string `json:"contracts"`
		Path      string   `json:"path"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out1)), &u); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out1)
	}
	if u.ID != "demo.crit" {
		t.Errorf("next head = %q, want demo.crit (critical outranks low)", u.ID)
	}
	if u.Goal == "" || u.Severity != "critical" || u.Path == "" {
		t.Errorf("start-context fields incomplete: %+v", u)
	}
	if len(u.Contracts) != 1 || u.Contracts[0] != "demo.c1" {
		t.Errorf("contracts = %v, want [demo.c1]", u.Contracts)
	}
}

func TestNext_JSON_EmptyObjectWhenNoReady(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	out := execCmd(t, "next", "--json")
	if strings.TrimSpace(out) != "{}" {
		t.Errorf("no-ready next --json = %q, want {}", strings.TrimSpace(out))
	}
}
