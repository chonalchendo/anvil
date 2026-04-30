package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestProject_Current(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(dir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"project", "current"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "foo") {
		t.Errorf("got %q", out.String())
	}
}

func TestProject_Current_JSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(dir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"project", "current", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["slug"] != "foo" {
		t.Errorf("slug = %q", got["slug"])
	}
}

func TestProject_AdoptAndList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(dir)

	adopt := newRootCmd()
	adopt.SetArgs([]string{"project", "adopt", "custom"})
	if err := adopt.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"project", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "custom") {
		t.Errorf("expected 'custom' in list output:\n%s", out.String())
	}
}

func TestProject_Switch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(dir)
	if err := core.AdoptProject("foo"); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"project", "switch", "foo"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
