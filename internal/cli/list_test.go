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

func TestList_Learning_MultiTagAllOf(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	mustCreateLearning := func(title string, tags []string, diataxis, confidence string) {
		t.Helper()
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "learning", "--title", title})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create %q: %v", title, err)
		}
		path := filepath.Join(vault, "20-learnings", core.Slugify(title)+".md")
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		anyTags := make([]any, 0, len(tags))
		for _, tag := range tags {
			anyTags = append(anyTags, tag)
		}
		a.FrontMatter["tags"] = anyTags
		a.FrontMatter["diataxis"] = diataxis
		a.FrontMatter["confidence"] = confidence
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}

	mustCreateLearning("alpha",
		[]string{"type/learning", "domain/postgres", "activity/debugging"}, "reference", "high")
	mustCreateLearning("beta",
		[]string{"type/learning", "domain/postgres"}, "explanation", "low")
	mustCreateLearning("gamma",
		[]string{"type/learning", "domain/typescript", "activity/debugging"}, "reference", "high")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "learning",
		"--tags", "domain/postgres,activity/debugging",
		"--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["id"] != "alpha" {
		t.Errorf("--tags all-of: got %v, want [alpha]", items)
	}
}

func TestList_Learning_DiataxisAndConfidence(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	_ = vault

	mustCreateLearning := func(title, diataxis, confidence string) {
		t.Helper()
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "learning", "--title", title})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create %q: %v", title, err)
		}
		path := filepath.Join(vault, "20-learnings", core.Slugify(title)+".md")
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["diataxis"] = diataxis
		a.FrontMatter["confidence"] = confidence
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}

	mustCreateLearning("ref-high", "reference", "high")
	mustCreateLearning("ref-low", "reference", "low")
	mustCreateLearning("exp-high", "explanation", "high")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "learning",
		"--diataxis", "reference", "--confidence", "high", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["id"] != "ref-high" {
		t.Errorf("got %v, want [ref-high]", items)
	}
}
