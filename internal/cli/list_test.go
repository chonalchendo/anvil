package cli

import (
	"bytes"
	"path/filepath"
	"strings"
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
	out, _, err := runCmd(t, cmd, "list", "issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 2 || env.Returned != 2 {
		t.Errorf("total=%d returned=%d, want 2/2", env.Total, env.Returned)
	}
	if env.Truncated {
		t.Error("expected truncated=false")
	}
	ids := []string{env.Items[0].ID, env.Items[1].ID}
	// Ties on created -> ID asc; both fixtures share the same created date.
	if ids[0] != "foo.a" || ids[1] != "foo.b" {
		t.Errorf("ids = %v, want [foo.a foo.b]", ids)
	}
	for _, item := range env.Items {
		if item.ID == "" || item.Type == "" || item.Title == "" || item.Status == "" || item.Path == "" {
			t.Errorf("missing required field in %+v", item)
		}
	}
}

func TestList_Empty(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	out, errOut, err := runCmd(t, cmd, "list", "issue")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("expected empty stdout, got %q", out)
	}
	if errOut != "" {
		t.Errorf("expected empty stderr, got %q", errOut)
	}

	cmd2 := newRootCmd()
	out2, _, err := runCmd(t, cmd2, "list", "issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out2)
	if len(env.Items) != 0 || env.Total != 0 || env.Returned != 0 || env.Truncated {
		t.Errorf("expected empty envelope, got %+v", env)
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
	out, _, err := runCmd(t, cmd, "list", "learning",
		"--tags", "domain/postgres,activity/debugging", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if len(env.Items) != 1 || env.Items[0].ID != "alpha" {
		t.Errorf("--tags all-of: got %+v, want [alpha]", env.Items)
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
	out, _, err := runCmd(t, cmd, "list", "learning",
		"--diataxis", "reference", "--confidence", "high", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if len(env.Items) != 1 || env.Items[0].ID != "ref-high" {
		t.Errorf("got %+v, want [ref-high]", env.Items)
	}
}

func TestList_LimitDefault10(t *testing.T) {
	newTestVaultWithIssues(t, 15)
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Returned != 10 {
		t.Errorf("returned=%d want 10", env.Returned)
	}
	if env.Total != 15 {
		t.Errorf("total=%d want 15", env.Total)
	}
	if !env.Truncated {
		t.Error("expected truncated=true")
	}
}

func TestList_RecencySort(t *testing.T) {
	newTestVaultWithDatedIssues(t, []string{"2026-05-01", "2026-05-03", "2026-05-02"})
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Items[0].Created != "2026-05-03" {
		t.Errorf("expected most recent first, got %s", env.Items[0].Created)
	}
}

func TestList_SinceFilter(t *testing.T) {
	newTestVaultWithDatedIssues(t, []string{"2026-04-30", "2026-05-02", "2026-05-04"})
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "issue", "--since", "2026-05-01", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 2 {
		t.Errorf("total=%d want 2", env.Total)
	}
}

func TestList_TruncationHintOnStderr(t *testing.T) {
	newTestVaultWithIssues(t, 15)
	cmd := newRootCmd()
	_, errOut, err := runCmd(t, cmd, "list", "issue", "--limit", "5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut, "showing 5 of 15") {
		t.Errorf("expected truncation hint, got %q", errOut)
	}
}

func TestList_NoHintWhenComplete(t *testing.T) {
	newTestVaultWithIssues(t, 3)
	cmd := newRootCmd()
	_, errOut, err := runCmd(t, cmd, "list", "issue")
	if err != nil {
		t.Fatal(err)
	}
	if errOut != "" {
		t.Errorf("expected empty stderr, got %q", errOut)
	}
}

func TestList_JSONItemFields(t *testing.T) {
	newTestVaultWithIssues(t, 1)
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if len(env.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(env.Items))
	}
	item := env.Items[0]
	if item.Description == "" {
		t.Error("description missing")
	}
	if item.Created == "" {
		t.Error("created missing")
	}
	if item.Project == "" {
		t.Error("project missing")
	}
}

func TestListInbox_NonEmpty(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "inbox"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestListInbox_LimitAndSince(t *testing.T) {
	newTestVaultWithDatedInbox(t, []string{"2026-04-30", "2026-05-02", "2026-05-04"})
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "list", "inbox", "--since", "2026-05-01", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 2 {
		t.Errorf("total=%d want 2", env.Total)
	}
}
