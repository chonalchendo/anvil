package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runNoVault(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestNoVault_VerbsFailNamingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "typo")
	t.Setenv("ANVIL_VAULT", root)
	for _, args := range [][]string{
		{"next"},
		{"list", "milestone"},
		{"tags", "list"},
		{"doctor"},
		{"create", "inbox", "--title", "x", "--suggested-type", "issue"},
	} {
		out, err := runNoVault(t, "", args...)
		if err == nil || !strings.Contains(err.Error(), root) {
			t.Errorf("%v: err = %v, out = %s; want error naming %s", args, err, out, root)
		}
	}
	if _, err := os.Stat(root); err == nil {
		t.Errorf("a failed verb conjured %s", root)
	}
}

func TestNoVault_HooksExitSilentlyAndCreateNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANVIL_VAULT", "")
	payload := `{"session_id":"abc","source":"resume"}`
	for _, hook := range []string{"fire-session-start", "fire-session-resume"} {
		out, err := runNoVault(t, payload, "install", hook)
		if err != nil || out != "" {
			t.Errorf("%s: err = %v, out = %q; want silent success", hook, err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "anvil-vault")); err == nil {
		t.Error("hook conjured $HOME/anvil-vault")
	}
}

func TestWhere_FlagsMissingVault(t *testing.T) {
	t.Setenv("ANVIL_VAULT", filepath.Join(t.TempDir(), "typo"))
	out, err := runNoVault(t, "", "where")
	if err != nil || !strings.Contains(out, "(missing") {
		t.Errorf("where: err = %v, out = %q; want missing note", err, out)
	}
}
