package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCreate_NearDuplicate_Surfaces_PriorIssue_JSON(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	for _, args := range [][]string{
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Improve foo bar", "--description", "near dup", "--tags", "domain/dev-tools", "--json"},
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
		if w["id"] != "foo.improve-foo-bar-baz" {
			t.Errorf("warning id = %v, want foo.improve-foo-bar-baz", w["id"])
		}
	}
}

func TestCreate_NearDuplicate_Surfaces_PriorIssue_Text(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	for _, args := range [][]string{
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Improve foo bar", "--description", "near dup", "--tags", "domain/dev-tools"},
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
		if !bytesContains(errBuf.Bytes(), []byte("foo.improve-foo-bar-baz")) {
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
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Improve foo bar", "--description", "near dup", "--tags", "domain/dev-tools", "--force-new"},
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
		{"create", "issue", "--title", "Improve foo bar baz", "--description", "first", "--tags", "domain/dev-tools", "--allow-new-facet=domain"},
		{"create", "issue", "--title", "Totally unrelated thing", "--description", "no overlap", "--tags", "domain/dev-tools"},
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

func bytesContains(b, sub []byte) bool { return bytes.Contains(b, sub) }
