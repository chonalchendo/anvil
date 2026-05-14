package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestFlagLeadingErrRE(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"--body and --no-body are mutually exclusive", true},
		{"--description too long: 160 chars (max 120)", true},
		{"-f requires an argument", true},
		{"---weird but still flag-shaped", true},
		{"flag --body conflicts with --no-body", false},
		{"unknown command", false},
		{"-- needs a letter", false},
		{"-1 leading digit, not a flag", false},
		{"", false},
	}
	for _, c := range cases {
		if got := flagLeadingErrRE.MatchString(c.in); got != c.want {
			t.Errorf("flagLeadingErrRE.MatchString(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRoot_VaultFlagOverridesEnvAndCwd(t *testing.T) {
	flagDir := t.TempDir()
	envDir := t.TempDir()

	// Seed envDir with a real issue so we can detect which vault was read.
	if err := (&core.Vault{Root: envDir}).Scaffold(); err != nil {
		t.Fatal(err)
	}
	issuePath := filepath.Join(envDir, "70-issues", "foo.bar.md")
	a := &core.Artifact{
		Path: issuePath,
		FrontMatter: map[string]any{
			"type":        "issue",
			"title":       "x",
			"description": "d",
			"created":     "2026-04-29",
			"status":      "open",
			"project":     "foo",
			"severity":    "low",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANVIL_VAULT", envDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--vault", flagDir, "list", "issue", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// flagDir is empty — total should be 0
	if !strings.Contains(out.String(), `"total":0`) {
		t.Errorf("flag should override env; expected total:0 from empty flag-vault, got %q", out.String())
	}
}

func TestRoot_VaultEnvOverridesCwdFallback(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("ANVIL_VAULT", envDir)
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("env should resolve vault: %v", err)
	}
	if !strings.Contains(out.String(), `"total":0`) {
		t.Errorf("expected envelope, got %q", out.String())
	}
}
