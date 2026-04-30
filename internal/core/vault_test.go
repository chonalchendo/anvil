package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultResolve_DefaultsToHome(t *testing.T) {
	t.Setenv("ANVIL_VAULT", "")
	t.Setenv("HOME", t.TempDir())
	v, err := ResolveVault()
	if err != nil {
		t.Fatalf("ResolveVault: %v", err)
	}
	want := filepath.Join(os.Getenv("HOME"), "anvil-vault")
	if v.Root != want {
		t.Errorf("Root = %q, want %q", v.Root, want)
	}
}

func TestVaultResolve_RespectsEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANVIL_VAULT", dir)
	v, err := ResolveVault()
	if err != nil {
		t.Fatalf("ResolveVault: %v", err)
	}
	if v.Root != dir {
		t.Errorf("Root = %q, want %q", v.Root, dir)
	}
}

func TestVaultScaffold_CreatesAllDirs(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	for _, d := range VaultDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("missing dir %q: %v", d, err)
		}
	}
}

func TestVaultScaffold_Idempotent(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, "00-inbox", "user-note.md")
	if err := os.WriteFile(probe, []byte("hand-written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Scaffold(); err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}
	got, err := os.ReadFile(probe)
	if err != nil || string(got) != "hand-written" {
		t.Errorf("user file modified or removed: %s, %v", got, err)
	}
}
