package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreate_NearDuplicate_Surfaces_PriorIssue_JSON(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	for _, args := range [][]string{
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--goal", "foo bar baz is improved", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Improve foo bar", "--description", "near dup", "--goal", "foo bar is improved", "--tags", "domain/dev-tools", "--json"},
	} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create %v: %v\nstdout: %s\nstderr: %s", args, err, out.String(), errBuf.String())
		}
		if args[len(args)-1] != "--json" {
			continue
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("parse json: %v\nout: %s", err, out.String())
		}
		warnings, _ := got["warnings"].([]any)
		if len(warnings) != 1 {
			t.Fatalf("warnings = %v, want 1 entry surfacing the prior id", got["warnings"])
		}
		w, _ := warnings[0].(map[string]any)
		// Numbered format: foo.NNNN.improve-foo-bar-baz
		wid, _ := w["id"].(string)
		if !strings.HasPrefix(wid, "foo.") || !strings.Contains(wid, "improve-foo-bar-baz") {
			t.Errorf("warning id = %v, want foo.NNNN.improve-foo-bar-baz", w["id"])
		}
	}
}

func TestCreate_NearDuplicate_Surfaces_PriorIssue_Text(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	for _, args := range [][]string{
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--goal", "foo bar baz is improved", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Improve foo bar", "--description", "near dup", "--goal", "foo bar is improved", "--tags", "domain/dev-tools"},
	} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create %v: %v", args, err)
		}
		if args[2] != "Improve foo bar" {
			continue
		}
		if !bytesContains(errBuf.Bytes(), []byte("improve-foo-bar-baz")) {
			t.Errorf("stderr missing prior id; got: %s", errBuf.String())
		}
		if !bytesContains(errBuf.Bytes(), []byte("similar")) {
			t.Errorf("stderr missing 'similar' marker; got: %s", errBuf.String())
		}
	}
}

func TestCreate_NearDuplicate_ForceNew_SkipsCheck(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	for _, args := range [][]string{
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--goal", "foo bar baz is improved", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Improve foo bar", "--description", "near dup", "--goal", "foo bar is improved", "--tags", "domain/dev-tools", "--force-new"},
	} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create: %v", err)
		}
		if args[len(args)-1] == "--force-new" && bytesContains(errBuf.Bytes(), []byte("similar")) {
			t.Errorf("--force-new should suppress similarity warning; stderr: %s", errBuf.String())
		}
	}
}

func TestCreate_NearDuplicate_NoMatch_NoWarning(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	for _, args := range [][]string{
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--goal", "foo bar baz is improved", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Totally unrelated thing", "--description", "no overlap", "--goal", "totally unrelated thing is done", "--tags", "domain/dev-tools"},
	} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create: %v", err)
		}
		if args[2] == "Totally unrelated thing" && bytesContains(errBuf.Bytes(), []byte("similar")) {
			t.Errorf("unrelated title should not warn; stderr: %s", errBuf.String())
		}
	}
}

func TestSimilarSlugs_OverlapCoefficient(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"improve-foo-bar-baz", "improve-foo-bar", true},
		{"taskcreate-reminders-churn-context", "taskcreate-reminders-noisy", true},
		{"add-login-button", "totally-unrelated-thing", false},
		{"x", "y", false},
	}
	for _, tc := range cases {
		got := similarSlugs(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("similarSlugs(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestCreate_ContentDuplicate_DisjointTitles verifies that two milestones with
// disjoint title tokens but identical description+goal are flagged as near-duplicates
// via FTS content search. Slug overlap alone would not fire on these titles.
func TestCreate_ContentDuplicate_DisjointTitles(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	// First milestone: title tokens are "reindex", "drops", "links", "concurrent", "writes".
	firstArgs := []string{
		"create", "milestone",
		"--project", "foo",
		"--title", "Reindex drops links on concurrent writes",
		"--description", "concurrent saves lose graph edges in the index",
		"--goal", "concurrent index writes no longer drop link rows",
		"--json",
	}
	// Second milestone: title tokens are "index", "loses", "edges", "under",
	// "parallel", "saves" — overlap with first title is zero significant tokens,
	// but description+goal are identical.
	secondArgs := []string{
		"create", "milestone",
		"--project", "foo",
		"--title", "Index loses edges under parallel saves",
		"--description", "concurrent saves lose graph edges in the index",
		"--goal", "concurrent index writes no longer drop link rows",
		"--json",
	}

	for i, args := range [][]string{firstArgs, secondArgs} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create #%d: %v\nstdout: %s\nstderr: %s", i+1, err, out.String(), errBuf.String())
		}
		if i == 0 {
			continue // first create has no prior to match
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("parse json: %v\nout: %s", err, out.String())
		}
		warnings, _ := got["warnings"].([]any)
		if len(warnings) == 0 {
			t.Fatalf("content-duplicate milestone: expected warnings but got none\nout: %s\nstderr: %s", out.String(), errBuf.String())
		}
	}
}

// TestCreate_ContentDuplicate_PriorNotBootstrapReindexed verifies the create-time
// FTS population path: a third milestone duplicating the SECOND (not the first)
// is flagged even though no `anvil reindex` ran between the second create and the
// third. The first create incidentally bootstrap-reindexes the whole vault (its
// stamp was unset), so the first row lands in artifact_fts for free. The second
// create does NOT bootstrap-reindex (stamp now set) — its artifact_fts row exists
// only because indexAfterSave now calls IndexArtifactFTS. The third milestone's
// content matches only the second (disjoint from the first's content AND title),
// so a missing second-row FTS entry would silently drop the warning.
func TestCreate_ContentDuplicate_PriorNotBootstrapReindexed(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	// #1: distinct content X — incidentally bootstrap-reindexed into artifact_fts.
	first := []string{
		"create", "milestone", "--project", "foo",
		"--title", "Reindex drops links on concurrent writes",
		"--description", "concurrent saves lose graph edges in the index",
		"--goal", "concurrent index writes no longer drop link rows",
		"--json",
	}
	// #2: distinct content Y, disjoint title and content from #1. Created with the
	// stamp already set, so it is FTS-indexed only via the create-time hook.
	second := []string{
		"create", "milestone", "--project", "foo",
		"--title", "Validate rejects malformed frontmatter keys",
		"--description", "schema validation skips unknown top-level fields entirely",
		"--goal", "validation flags every unexpected frontmatter key as an error",
		"--json",
	}
	// #3: content Y again, title disjoint from #2 — matches ONLY #2 by content.
	third := []string{
		"create", "milestone", "--project", "foo",
		"--title", "Frontmatter typos slip past the checker",
		"--description", "schema validation skips unknown top-level fields entirely",
		"--goal", "validation flags every unexpected frontmatter key as an error",
		"--json",
	}

	for i, args := range [][]string{first, second, third} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create #%d: %v\nstdout: %s\nstderr: %s", i+1, err, out.String(), errBuf.String())
		}
		if i < 2 {
			continue
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("parse json: %v\nout: %s", err, out.String())
		}
		warnings, _ := got["warnings"].([]any)
		if len(warnings) == 0 {
			t.Fatalf("third milestone duplicating the non-reindexed second: expected a warning but got none\nout: %s\nstderr: %s", out.String(), errBuf.String())
		}
	}
}

func bytesContains(b, sub []byte) bool { return bytes.Contains(b, sub) }
