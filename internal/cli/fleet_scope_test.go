package cli

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestScopeAudit_ViolationsDetected(t *testing.T) {
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

func TestScopeAudit_Clean(t *testing.T) {
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
