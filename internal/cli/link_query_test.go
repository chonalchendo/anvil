package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkFromReturnsOutgoingEdges(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "a", "--description", "a",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "b", "--description", "b",
		"--tags", "domain/dev-tools")
	execCmd(t, "link", "issue", "demo.a", "issue", "demo.b")

	out := execCmd(t, "link", "--from", "demo.a", "--json")
	var rows []struct {
		Source, Target, Relation, Path string
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rows); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if len(rows) != 1 || rows[0].Target != "demo.b" || !strings.HasSuffix(rows[0].Path, "demo.a.md") {
		t.Fatalf("rows: %v", rows)
	}
}

func TestLinkToReturnsIncomingEdges(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "a", "--description", "a",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "b", "--description", "b",
		"--tags", "domain/dev-tools")
	execCmd(t, "link", "issue", "demo.a", "issue", "demo.b")

	out := execCmd(t, "link", "--to", "demo.b", "--json")
	var rows []struct {
		Source, Target, Relation string
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rows); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if len(rows) != 1 || rows[0].Source != "demo.a" {
		t.Fatalf("rows: %v", rows)
	}
}

func TestLinkUnresolvedReturnsDanglingEdges(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// Manually write an issue whose milestone wikilink points at a missing id.
	if err := os.MkdirAll(filepath.Join(vault, "70-issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: issue\nid: demo.foo\nstatus: open\nmilestone: \"[[milestone.demo.gone]]\"\n---\n\n"
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.foo.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	out := execCmd(t, "link", "--unresolved", "--json")
	var rows []struct {
		Source, Target string
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rows); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if len(rows) != 1 || rows[0].Target != "demo.gone" {
		t.Fatalf("rows: %v", rows)
	}
}

func TestLinkDriftFlagsSlugMismatch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	// One drift pair: plan slug `pre-parse` links to issue `with-pre-parse`.
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "x", "--description", "x",
		"--slug", "with-pre-parse",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "plan",
		"--project", "demo", "--title", "y", "--description", "y",
		"--slug", "pre-parse",
		"--issue", "[[issue.demo.with-pre-parse]]",
		"--tags", "domain/dev-tools")

	// One clean pair: plan slug matches issue slug exactly.
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "z", "--description", "z",
		"--slug", "aligned",
		"--tags", "domain/dev-tools")
	execCmd(t, "create", "plan",
		"--project", "demo", "--title", "w", "--description", "w",
		"--issue", "[[issue.demo.aligned]]",
		"--tags", "domain/dev-tools")

	out := execCmd(t, "link", "--drift", "--json")
	var rows []map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rows); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 drift row, got %d: %s", len(rows), out)
	}
	r := rows[0]
	if r["source"] != "demo.pre-parse" || r["target"] != "demo.with-pre-parse" {
		t.Errorf("source/target = %q/%q", r["source"], r["target"])
	}
	if r["source_slug"] != "pre-parse" || r["target_slug"] != "with-pre-parse" {
		t.Errorf("source_slug/target_slug = %q/%q", r["source_slug"], r["target_slug"])
	}
}

func TestLinkReadModesMutuallyExclusiveWithWriteForm(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "--from", "demo.a", "issue", "demo.x", "issue", "demo.y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error, got: %s", out.String())
	}
}
