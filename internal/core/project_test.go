package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitInit(t *testing.T, dir, remote string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", remote},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestResolveProject_FromGitRemote(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "git@github.com:acme/payment-service.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(dir)
	p, err := ResolveProject()
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if p.Slug != "payment-service" {
		t.Errorf("Slug = %q, want payment-service", p.Slug)
	}
}

func TestResolveProject_NoRemote_NoBinding_Errors(t *testing.T) {
	dir := t.TempDir()
	c := exec.Command("git", "init", "-q")
	c.Dir = dir
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(dir)
	_, err := ResolveProject()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProject_Adopt_WritesBinding(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "git@github.com:acme/foo.git")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(dir)
	if err := AdoptProject("custom-slug"); err != nil {
		t.Fatalf("AdoptProject: %v", err)
	}
	bp := filepath.Join(home, ".anvil", "projects", "custom-slug", ".binding")
	if _, err := os.Stat(bp); err != nil {
		t.Errorf("binding not written at %s: %v", bp, err)
	}
	p, err := ResolveProject()
	if err != nil || p.Slug != "custom-slug" {
		t.Errorf("after adopt, slug = %v / err %v", p, err)
	}
}
