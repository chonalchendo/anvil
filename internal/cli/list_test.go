package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestList_FiltersByStatus(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A issue")
	writeFixtureIssue(t, vault, "foo", "b", "B issue")
	// Mark second as resolved.
	pathB := filepath.Join(vault, "70-issues", "foo.b.md")
	a, _ := core.LoadArtifact(pathB)
	a.FrontMatter["status"] = "resolved"
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue", "--status", "open"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !bytes.Contains(out.Bytes(), []byte("foo.a")) {
		t.Errorf("foo.a missing:\n%s", got)
	}
	if bytes.Contains(out.Bytes(), []byte("foo.b")) {
		t.Errorf("foo.b should be filtered out:\n%s", got)
	}
}

func TestList_JSON(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A issue")
	writeFixtureIssue(t, vault, "foo", "b", "B issue")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	if got[0]["id"] != "foo.a" || got[1]["id"] != "foo.b" {
		t.Errorf("ids = %v, %v", got[0]["id"], got[1]["id"])
	}
	for _, item := range got {
		for _, k := range []string{"id", "type", "title", "status", "path"} {
			if _, ok := item[k]; !ok {
				t.Errorf("missing key %q in %v", k, item)
			}
		}
	}
}

func TestList_Empty(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("expected empty output, got %q", out.String())
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"list", "issue", "--json"})
	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
	if s := bytes.TrimSpace(out2.Bytes()); string(s) != "[]" {
		t.Errorf("expected [], got %q", s)
	}
}

func TestList_TagSubstring(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A issue")
	// Append a tag
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	a.FrontMatter["tags"] = []any{"type/issue", "domain/auth"}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue", "--tag", "auth"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("foo.a")) {
		t.Errorf("expected foo.a in output:\n%s", out.String())
	}
}
