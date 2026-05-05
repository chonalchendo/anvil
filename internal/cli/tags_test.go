package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// writeArtifact drops a frontmatter-only markdown file into the vault for the test.
func writeArtifact(t *testing.T, root, rel string, fm string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" + fm + "---\n\n# body\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTagsList_Aggregates(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [domain/dev-tools, activity/research]\n")
	writeArtifact(t, root, "20-learnings/anvil.b.md",
		"type: learning\ntitle: B\ntags: [domain/dev-tools]\n")
	writeArtifact(t, root, "70-issues/anvil.c.md",
		"type: issue\ntitle: C\ntags: [domain/dev-tools, type/issue]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var got []struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse json: %v\nraw: %s", err, out.String())
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Tag < got[j].Tag })

	want := []struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}{
		{Tag: "activity/research", Count: 1},
		{Tag: "domain/dev-tools", Count: 3},
		{Tag: "type/issue", Count: 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("tags mismatch (-want +got):\n%s", diff)
	}
}

func TestTagsList_FilterByType(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [foo, bar]\n")
	writeArtifact(t, root, "70-issues/anvil.b.md",
		"type: issue\ntitle: B\ntags: [foo, baz]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--type", "learning", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), `"bar"`) || strings.Contains(out.String(), `"baz"`) {
		t.Errorf("filter by --type learning leaked issue tags: %s", out.String())
	}
}

func TestTagsList_FilterByPrefix(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [domain/x, activity/y, type/learning]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--prefix", "domain/", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), `"domain/x"`) ||
		strings.Contains(out.String(), `"activity/y"`) {
		t.Errorf("prefix filter wrong: %s", out.String())
	}
}

func TestTagsList_TextOutput(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [foo, foo-extra]\n")
	writeArtifact(t, root, "20-learnings/anvil.b.md",
		"type: learning\ntitle: B\ntags: [foo]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Format: "<count>\t<tag>\n", sorted by tag.
	want := "1\tfoo-extra\n2\tfoo\n"
	got := out.String()
	// Order may be by descending count then tag — accept either documented order.
	if got != want && got != "2\tfoo\n1\tfoo-extra\n" {
		t.Errorf("text output unexpected:\nwant one of:\n%q\n%q\ngot:\n%q",
			want, "2\tfoo\n1\tfoo-extra\n", got)
	}
}
