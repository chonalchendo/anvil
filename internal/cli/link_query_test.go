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
