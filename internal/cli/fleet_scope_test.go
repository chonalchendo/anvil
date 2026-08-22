package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// disableDeriveChanged stubs deriveChangedFn to always fail, so a CLI-level
// scope-audit test exercises the caller-supplied --changed list in
// isolation — it must not pick up this repo's own real git diff (this
// package's working tree, not the test's fixture) when the derivation seam
// runs unstubbed.
func disableDeriveChanged(t *testing.T) {
	t.Helper()
	orig := deriveChangedFn
	deriveChangedFn = func() ([]string, string, error) { return nil, "", errors.New("stubbed: no derivation") }
	t.Cleanup(func() { deriveChangedFn = orig })
}

func TestScopeAudit_ViolationsDetected(t *testing.T) {
	disableDeriveChanged(t)
	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "fleet", "scope-audit",
		"--declared", "a.py,b.py",
		"--changed", "a.py,cli.py",
	)
	if err != nil {
		t.Fatalf("scope-audit: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "cli.py") {
		t.Errorf("expected cli.py in output, got: %q", stdout)
	}
	if strings.Contains(stdout, "a.py") {
		t.Errorf("a.py is declared; must not appear in output, got: %q", stdout)
	}
}

func TestScopeAudit_InvalidDeclaredRejected(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		wantSub  string
	}{
		{"prose-estimate", "pkg models audits tests for file", "pkg models audits tests for file"},
		{"carriage-return", "pkg\rmodels", `pkg\rmodels`},
		{"unsubstituted-placeholder", "<declared-files>", "<declared-files>"},
		{"empty", "", "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()
			stdout, stderr, err := runCmd(t, cmd, "fleet", "scope-audit",
				"--declared", tc.declared,
				"--changed", "pkg/file.py",
			)
			if err == nil {
				t.Fatalf("expected error for invalid declared set, got none; stdout=%q", stdout)
			}
			if combined := err.Error() + stderr; !strings.Contains(combined, tc.wantSub) {
				t.Errorf("expected error to contain %q, got: %q", tc.wantSub, combined)
			}
		})
	}
}

func TestScopeAudit_Clean(t *testing.T) {
	disableDeriveChanged(t)
	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "fleet", "scope-audit",
		"--declared", "a.py,b.py",
		"--changed", "a.py,b.py",
	)
	if err != nil {
		t.Fatalf("scope-audit: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "clean") {
		t.Errorf("expected 'clean' in output, got: %q", stdout)
	}
}

func TestScopeViolations(t *testing.T) {
	cases := []struct {
		name     string
		declared []string
		changed  []string
		want     []string
	}{
		{"all-in-scope", []string{"a.py", "b.py"}, []string{"a.py", "b.py"}, nil},
		{"one-violation", []string{"a.py", "b.py"}, []string{"a.py", "cli.py"}, []string{"cli.py"}},
		{"no-declared", nil, []string{"x.go"}, []string{"x.go"}},
		{"empty-changed", []string{"a.py"}, nil, nil},
		{"multiple-violations", []string{"a.py"}, []string{"b.py", "c.py"}, []string{"b.py", "c.py"}},
		{"single-star", []string{"internal/cli/*.go"}, []string{"internal/cli/fleet.go"}, nil},
		{
			"multi-star",
			[]string{"a/test_*industry*.yaml"},
			[]string{"a/test_agg_fundamentals__industry_valuation.yaml", "b/other.py"},
			[]string{"b/other.py"},
		},
		{"question-mark", []string{"a/v?.sql"}, []string{"a/v1.sql", "a/v10.sql"}, []string{"a/v10.sql"}},
		{
			"brace-alternation",
			[]string{"a/met_{growth,health}.sql"},
			[]string{"a/met_growth.sql", "a/met_health.sql", "a/met_valuation.sql"},
			[]string{"a/met_valuation.sql"},
		},
		{"brace-with-star", []string{"a/{x,y}_*.sql"}, []string{"a/y_rel.sql"}, nil},
		{"star-does-not-cross-separator", []string{"a/*.go"}, []string{"a/b/c.go"}, []string{"a/b/c.go"}},
		{"unbalanced-brace-is-literal", []string{"a/{x.go"}, []string{"a/{x.go", "a/x.go"}, []string{"a/x.go"}},
		{"declared-dir-trailing-slash", []string{"pkg/"}, []string{"pkg/src/mod.py"}, nil},
		{"declared-dir-bare-root", []string{"pkg"}, []string{"pkg/src/mod.py"}, nil},
		{"declared-dir-covers-any-depth", []string{"tests"}, []string{"tests/a/b/c_test.py"}, nil},
		{"declared-dir-brace-alternation", []string{"internal/{build,cli}/"}, []string{"internal/cli/x.go", "internal/schema/y.go"}, []string{"internal/schema/y.go"}},
		{"declared-dir-excludes-sibling", []string{"pkg/"}, []string{"other/file.py"}, []string{"other/file.py"}},
		{
			"declared-dir-is-not-a-substring",
			[]string{"pkg"},
			[]string{"pkgtools/x.go", "src/pkg/y.go"},
			[]string{"pkgtools/x.go", "src/pkg/y.go"},
		},
		{"declared-file-is-not-a-prefix", []string{"a/b.go"}, []string{"a/b.go.bak"}, []string{"a/b.go.bak"}},
		{"declared-dotted-dir", []string{".github"}, []string{".github/workflows/ci.yml"}, nil},
		{"declared-dotted-dir-trailing-slash", []string{".github/"}, []string{".github/workflows/ci.yml"}, nil},
		{"declared-dotted-dir-excludes-sibling", []string{".github"}, []string{".githubbed/x.yml"}, []string{".githubbed/x.yml"}},
		{"declared-dot-in-mid-path-dir", []string{"a.b/c"}, []string{"a.b/c/d.go"}, nil},
		{"empty-pattern-covers-nothing", []string{"{,pkg}"}, []string{"other/x.go"}, []string{"other/x.go"}},
		{"empty-pattern-sibling-still-covers", []string{"{,pkg}"}, []string{"pkg/x.go"}, nil},
		{"dotdot-escapes-declared-dir", []string{"pkg"}, []string{"pkg/../../etc/shadow"}, []string{"pkg/../../etc/shadow"}},
		{"dotdot-resolving-back-inside-is-covered", []string{"pkg"}, []string{"pkg/sub/../x.go"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scopeViolations(tc.declared, tc.changed)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("scopeViolations (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExpandBraces(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"no-braces", "a/b.go", []string{"a/b.go"}},
		{"one-group", "met_{growth,health,valuation}.sql", []string{"met_growth.sql", "met_health.sql", "met_valuation.sql"}},
		{"two-groups", "{a,b}/{x,y}.go", []string{"a/x.go", "a/y.go", "b/x.go", "b/y.go"}},
		{"nested", "m_{a,b{c,d}}.sql", []string{"m_a.sql", "m_bc.sql", "m_bd.sql"}},
		{"empty-alternative", "f{,_test}.go", []string{"f.go", "f_test.go"}},
		{"unbalanced", "a/{x.go", []string{"a/{x.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, expandBraces(tc.in)); diff != "" {
				t.Errorf("expandBraces (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"", nil},
		{"a", []string{"a"}},
		{",a,", []string{"a"}},
		{"a/met_{growth,health}.sql,b.py", []string{"a/met_{growth,health}.sql", "b.py"}},
		{"a/{x,{y,z}}.sql", []string{"a/{x,{y,z}}.sql"}},
		{"a/}x,b.py", []string{"a/}x", "b.py"}},
		// An unbalanced `{` must not swallow the rest of the list.
		{"a/{x.go,b.py", []string{"a/{x.go", "b.py"}},
		{"a/{x,y.go,b.py,c.py", []string{"a/{x", "y.go", "b.py", "c.py"}},
	}
	for _, tc := range cases {
		got := splitCSV(tc.in)
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("splitCSV(%q) (-want +got):\n%s", tc.in, diff)
		}
	}
}

func TestSplitLiteralCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"", nil},
		{",a,", []string{"a"}},
		// A `{` in a real filename is not alternation on the changed side.
		{"a/{x.go,b/out_of_scope.go", []string{"a/{x.go", "b/out_of_scope.go"}},
	}
	for _, tc := range cases {
		got := splitLiteralCSV(tc.in)
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("splitLiteralCSV(%q) (-want +got):\n%s", tc.in, diff)
		}
	}
}

// A changed path containing `{` must split on the comma, so each file is
// audited on its own instead of merging into a token naming no file.
func TestScopeAudit_ChangedPathsAreLiteral(t *testing.T) {
	disableDeriveChanged(t)
	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "fleet", "scope-audit",
		"--declared", "a/{x.go",
		"--changed", "a/{x.go,b/out_of_scope.go",
	)
	if err != nil {
		t.Fatalf("scope-audit: unexpected error: %v", err)
	}
	if got, want := strings.TrimSpace(stdout), "b/out_of_scope.go"; got != want {
		t.Errorf("scope-audit stdout = %q, want %q", got, want)
	}
}

// A typo'd `{` in --declared must degrade to literal entries, not consume every
// following entry and flag correctly-declared files.
func TestScopeAudit_UnbalancedDeclaredBraceDegrades(t *testing.T) {
	disableDeriveChanged(t)
	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "fleet", "scope-audit",
		"--declared", "a/{x.go,b.py",
		"--changed", "b.py",
	)
	if err != nil {
		t.Fatalf("scope-audit: unexpected error: %v", err)
	}
	if got, want := strings.TrimSpace(stdout), "scope: clean"; got != want {
		t.Errorf("scope-audit stdout = %q, want %q", got, want)
	}
}

func TestMergeChanged(t *testing.T) {
	cases := []struct {
		name    string
		caller  []string
		derived []string
		want    []string
	}{
		{"both-empty", nil, nil, nil},
		{"caller-only", []string{"a.py"}, nil, []string{"a.py"}},
		{"derived-only", nil, []string{"a.py"}, []string{"a.py"}},
		{"union-dedup", []string{"a.py", "b.py"}, []string{"b.py", "c.py"}, []string{"a.py", "b.py", "c.py"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeChanged(tc.caller, tc.derived)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mergeChanged (-want +got):\n%s", diff)
			}
		})
	}
}

// TestScopeAudit_DerivedDiffCatchesStaleBaseUnderReport reproduces the PR
// #231 incident (issue.anvil.0236): a `git reset --soft` onto a newer base
// folds a reversion of landed work into a commit, and a stale three-dot diff
// against the old base is content-identical to the reverted file — so it
// never appears in the caller's --changed. The local main is left stale at
// the fork point, so the audit only catches the reversion by deriving the
// changed set against the fetched origin/main — never a local branch.
func TestScopeAudit_DerivedDiffCatchesStaleBaseUnderReport(t *testing.T) {
	bare := t.TempDir()
	if out, err := runIn(bare, "git", "init", "-q", "--bare", "."); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := runIn(dir, "git", args...)
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(dir+"/"+name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", ".")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("remote", "add", "origin", bare)
	run("checkout", "-qb", "main")
	writeFile("landed.txt", "a\n")
	run("add", ".")
	run("commit", "-qm", "base")
	run("checkout", "-qb", "feat")
	run("checkout", "-q", "main")
	writeFile("landed.txt", "fix\n")
	run("commit", "-qam", "landed fix")
	run("push", "-q", "origin", "main")
	// Local main goes stale at the fork point; only origin/main has the fix.
	run("reset", "-q", "--hard", "main~1")
	run("checkout", "-q", "feat")
	writeFile("mine.txt", "mine\n")
	run("add", "mine.txt")
	run("reset", "--soft", "origin/main")
	run("commit", "-qm", "work")

	staleChanged := strings.TrimSpace(run("diff", "--name-only", "main...HEAD"))
	if strings.Contains(staleChanged, "landed.txt") {
		t.Fatalf("fixture invariant broken: stale diff already reports landed.txt: %q", staleChanged)
	}

	t.Chdir(dir)
	cmd := newRootCmd()
	stdout, stderr, err := runCmd(t, cmd, "fleet", "scope-audit",
		"--declared", "mine.txt",
		"--changed", staleChanged,
	)
	if err != nil {
		t.Fatalf("scope-audit: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "landed.txt") {
		t.Errorf("expected landed.txt flagged as out-of-scope, got: %q", stdout)
	}
	if !strings.Contains(stderr, "origin/main") {
		t.Errorf("expected the derivation base ref on stderr, got: %q", stderr)
	}
}

// A failed derivation (here: not a git repo) must degrade loudly — warning on
// stderr, audit of the --changed list alone — never a bare "scope: clean".
func TestScopeAudit_DerivationFailureWarnsAndDegrades(t *testing.T) {
	t.Chdir(t.TempDir())
	cmd := newRootCmd()
	stdout, stderr, err := runCmd(t, cmd, "fleet", "scope-audit",
		"--declared", "a.py",
		"--changed", "a.py",
	)
	if err != nil {
		t.Fatalf("scope-audit: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "clean") {
		t.Errorf("expected 'clean' on stdout, got: %q", stdout)
	}
	if !strings.Contains(stderr, "changed-set derivation failed") {
		t.Errorf("expected degradation warning on stderr, got: %q", stderr)
	}
}
