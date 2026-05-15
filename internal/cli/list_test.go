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
		cmd.SetArgs([]string{
			"create", "learning", "--title", title,
			"--tags", "domain/dev-tools,activity/research",
			"--allow-new-facet=domain", "--allow-new-facet=activity",
		})
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
		// Direct .Save() bypasses indexAfterSave; refresh the stamp so the
		// next create's freshness check doesn't see this file as drift.
		reindexCmd := newRootCmd()
		reindexCmd.SetArgs([]string{"reindex"})
		reindexCmd.SetOut(&bytes.Buffer{})
		reindexCmd.SetErr(&bytes.Buffer{})
		if err := reindexCmd.Execute(); err != nil {
			t.Fatalf("reindex after direct save: %v", err)
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
		cmd.SetArgs([]string{
			"create", "learning", "--title", title,
			"--tags", "domain/dev-tools,activity/research",
			"--allow-new-facet=domain", "--allow-new-facet=activity",
		})
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
		reindexCmd := newRootCmd()
		reindexCmd.SetArgs([]string{"reindex"})
		reindexCmd.SetOut(&bytes.Buffer{})
		reindexCmd.SetErr(&bytes.Buffer{})
		if err := reindexCmd.Execute(); err != nil {
			t.Fatalf("reindex after direct save: %v", err)
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

// TestList_ProjectFlag_RejectedForUnsupportedTypes asserts that passing
// --project for a type whose schema rejects `project:` returns a clean
// structured error rather than silently returning empty results.
func TestList_ProjectFlag_RejectedForUnsupportedTypes(t *testing.T) {
	setupVault(t)
	for _, typ := range []string{"inbox", "session", "sweep", "thread"} {
		t.Run(typ, func(t *testing.T) {
			cmd := newRootCmd()
			_, errOut, err := runCmd(t, cmd, "list", typ, "--project", "anvil")
			if err == nil {
				t.Fatalf("expected error for --project on %s, got nil", typ)
			}
			if !strings.Contains(errOut, "unsupported_flag_for_type") {
				t.Errorf("stderr missing code: %q", errOut)
			}
			if !strings.Contains(errOut, `"flag":"project"`) {
				t.Errorf("stderr missing flag field: %q", errOut)
			}
			if !strings.Contains(errOut, `"suggest"`) {
				t.Errorf("stderr missing suggest field: %q", errOut)
			}
		})
	}
}

// TestList_SeverityFilter exercises the issue --severity flag end-to-end:
// exact-match filtering, composition with --ready (indexed path), and the
// bad_flag_value error for unknown enum values.
func TestList_SeverityFilter(t *testing.T) {
	vault := setupVault(t)

	mustWriteIssue := func(slug, severity string) {
		t.Helper()
		path := writeFixtureIssue(t, vault, "foo", slug, "title "+slug)
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["severity"] = severity
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteIssue("a", "high")
	mustWriteIssue("b", "low")
	mustWriteIssue("c", "high")

	t.Run("exact-match filter", func(t *testing.T) {
		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "list", "issue", "--severity", "high", "--json")
		if err != nil {
			t.Fatal(err)
		}
		env := unmarshalListEnvelope(t, out)
		if env.Total != 2 {
			t.Errorf("total=%d want 2", env.Total)
		}
		for _, item := range env.Items {
			if item.Severity != "high" {
				t.Errorf("item %s severity=%q want high", item.ID, item.Severity)
			}
		}
	})

	t.Run("composes with --ready", func(t *testing.T) {
		// Reindex so --ready (indexed path) sees the freshly-saved issues.
		reindex := newRootCmd()
		reindex.SetArgs([]string{"reindex"})
		reindex.SetOut(&bytes.Buffer{})
		reindex.SetErr(&bytes.Buffer{})
		if err := reindex.Execute(); err != nil {
			t.Fatalf("reindex: %v", err)
		}
		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "list", "issue", "--severity", "high", "--ready", "--json")
		if err != nil {
			t.Fatal(err)
		}
		env := unmarshalListEnvelope(t, out)
		if env.Total != 2 {
			t.Errorf("ready+high total=%d want 2", env.Total)
		}
		for _, item := range env.Items {
			if item.Severity != "high" {
				t.Errorf("item %s severity=%q want high", item.ID, item.Severity)
			}
		}
	})

	t.Run("rejects unknown value", func(t *testing.T) {
		cmd := newRootCmd()
		_, errOut, err := runCmd(t, cmd, "list", "issue", "--severity", "spicy")
		if err == nil {
			t.Fatal("expected error for unknown severity, got nil")
		}
		if !strings.Contains(errOut, "bad_flag_value") {
			t.Errorf("stderr missing code: %q", errOut)
		}
		if !strings.Contains(errOut, `"flag":"severity"`) {
			t.Errorf("stderr missing flag field: %q", errOut)
		}
		if !strings.Contains(errOut, `"allowed":["low","medium","high","critical"]`) {
			t.Errorf("stderr missing allowed enum: %q", errOut)
		}
	})
}

// TestList_MilestoneFilterAndProjection exercises the four acceptance
// criteria of the milestone-column feature: JSON projects the milestone
// slug, --milestone filters on exact slug match, default text output
// surfaces a milestone cell (em-dash when unset), and the filter composes
// with --ready (indexed path) + --status.
func TestList_MilestoneFilterAndProjection(t *testing.T) {
	vault := setupVault(t)

	writeIssueWithMilestone := func(slug, milestone string) {
		t.Helper()
		path := writeFixtureIssue(t, vault, "foo", slug, "title "+slug)
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		if milestone != "" {
			a.FrontMatter["milestone"] = "[[milestone." + milestone + "]]"
		}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	writeIssueWithMilestone("a", "foo.m1")
	writeIssueWithMilestone("b", "foo.m2")
	writeIssueWithMilestone("c", "") // no milestone

	t.Run("json projects milestone slug", func(t *testing.T) {
		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "list", "issue", "--json")
		if err != nil {
			t.Fatal(err)
		}
		env := unmarshalListEnvelope(t, out)
		got := map[string]string{}
		for _, it := range env.Items {
			got[it.ID] = it.Milestone
		}
		if got["foo.a"] != "foo.m1" || got["foo.b"] != "foo.m2" || got["foo.c"] != "" {
			t.Errorf("milestone projection mismatch: %+v", got)
		}
	})

	t.Run("--milestone filter narrows", func(t *testing.T) {
		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "list", "issue", "--milestone", "foo.m1", "--json")
		if err != nil {
			t.Fatal(err)
		}
		env := unmarshalListEnvelope(t, out)
		if env.Total != 1 || env.Items[0].ID != "foo.a" {
			t.Errorf("milestone filter: got %+v, want [foo.a]", env.Items)
		}
	})

	t.Run("default text output shows milestone column and em-dash", func(t *testing.T) {
		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "list", "issue")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "foo.m1") {
			t.Errorf("expected milestone slug foo.m1 in text output, got:\n%s", out)
		}
		if !strings.Contains(out, "—") {
			t.Errorf("expected em-dash for milestone-less issue in text output, got:\n%s", out)
		}
	})

	t.Run("composes with --ready and --status", func(t *testing.T) {
		// Reindex so --ready (indexed path) sees the freshly-saved issues.
		reindex := newRootCmd()
		reindex.SetArgs([]string{"reindex"})
		reindex.SetOut(&bytes.Buffer{})
		reindex.SetErr(&bytes.Buffer{})
		if err := reindex.Execute(); err != nil {
			t.Fatalf("reindex: %v", err)
		}
		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "list", "issue",
			"--ready", "--status", "open", "--milestone", "foo.m1", "--json")
		if err != nil {
			t.Fatal(err)
		}
		env := unmarshalListEnvelope(t, out)
		if env.Total != 1 || env.Items[0].ID != "foo.a" {
			t.Errorf("ready+status+milestone compose: got %+v, want [foo.a]", env.Items)
		}
		if env.Items[0].Milestone != "foo.m1" {
			t.Errorf("indexed-path milestone projection: got %q want foo.m1", env.Items[0].Milestone)
		}
	})
}

// TestList_ProjectFlag_AcceptedForSupportedTypes guards against over-eager
// rejection: the supported set (issue, plan, milestone, designs, learning,
// decision) must keep accepting --project without error.
func TestList_ProjectFlag_AcceptedForSupportedTypes(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A issue")
	for _, typ := range []string{"issue", "learning", "decision"} {
		t.Run(typ, func(t *testing.T) {
			cmd := newRootCmd()
			_, _, err := runCmd(t, cmd, "list", typ, "--project", "foo")
			if err != nil {
				t.Fatalf("expected --project to work for %s, got %v", typ, err)
			}
		})
	}
}

// TestList_LearningDecision_ProjectFilter exercises end-to-end filtering on
// the optional project field for learning + decision (AC #3).
func TestList_LearningDecision_ProjectFilter(t *testing.T) {
	vault := setupVault(t)

	writeFixture := func(typ, dir, id, project string) {
		t.Helper()
		fm := map[string]any{
			"type": typ, "title": id, "created": "2026-05-10", "updated": "2026-05-10",
			"tags": []any{"domain/methodology", "activity/testing"},
		}
		switch typ {
		case "learning":
			fm["status"] = "draft"
			fm["diataxis"] = "explanation"
			fm["confidence"] = "low"
		case "decision":
			fm["status"] = "proposed"
			fm["date"] = "2026-05-10"
			fm["description"] = "fixture decision"
		}
		if project != "" {
			fm["project"] = project
		}
		path := filepath.Join(vault, dir, id+".md")
		a := &core.Artifact{Path: path, FrontMatter: fm, Body: "body\n"}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}

	writeFixture("learning", "20-learnings", "l-burgh", "burgh")
	writeFixture("learning", "20-learnings", "l-anvil", "anvil")
	writeFixture("learning", "20-learnings", "l-unscoped", "")
	writeFixture("decision", "30-decisions", "d-burgh", "burgh")
	writeFixture("decision", "30-decisions", "d-anvil", "anvil")

	for _, tc := range []struct {
		typ      string
		want     string
		wantMiss string
	}{
		{"learning", "l-burgh", "l-anvil"},
		{"decision", "d-burgh", "d-anvil"},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			cmd := newRootCmd()
			out, _, err := runCmd(t, cmd, "list", tc.typ, "--project", "burgh", "--json")
			if err != nil {
				t.Fatalf("list %s --project burgh: %v", tc.typ, err)
			}
			env := unmarshalListEnvelope(t, out)
			if env.Total != 1 || len(env.Items) != 1 || env.Items[0].ID != tc.want {
				got := make([]string, 0, len(env.Items))
				for _, it := range env.Items {
					got = append(got, it.ID)
				}
				t.Errorf("expected only %q, got %v (total=%d)", tc.want, got, env.Total)
			}
		})
	}
}
