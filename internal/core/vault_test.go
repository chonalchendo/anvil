package core

import (
	"encoding/json"
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

func TestEnableObsidianCorePlugin_CreatesFileWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{Root: dir}
	if err := v.EnableObsidianCorePlugin("bases"); err != nil {
		t.Fatalf("EnableObsidianCorePlugin: %v", err)
	}
	if got := readCorePlugins(t, dir); !got["bases"] {
		t.Errorf("bases not enabled: %v", got)
	}
}

func TestEnableObsidianCorePlugin_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{Root: dir}
	obs := filepath.Join(dir, ".obsidian")
	if err := os.MkdirAll(obs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(obs, "core-plugins.json"), []byte(`{"graph":true,"bases":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.EnableObsidianCorePlugin("bases"); err != nil {
		t.Fatalf("EnableObsidianCorePlugin: %v", err)
	}
	got := readCorePlugins(t, dir)
	if !got["bases"] {
		t.Errorf("bases not enabled: %v", got)
	}
	if !got["graph"] {
		t.Errorf("existing plugin clobbered: %v", got)
	}
}

func readCorePlugins(t *testing.T, vaultRoot string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vaultRoot, ".obsidian", "core-plugins.json")) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("reading core-plugins.json: %v", err)
	}
	var plugins map[string]bool
	if err := json.Unmarshal(b, &plugins); err != nil {
		t.Fatalf("parsing core-plugins.json: %v", err)
	}
	return plugins
}

func TestVaultScaffold_Idempotent(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, "00-inbox", "user-note.md")
	if err := os.WriteFile(probe, []byte("hand-written"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	if err := v.Scaffold(); err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}
	got, err := os.ReadFile(probe) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil || string(got) != "hand-written" {
		t.Errorf("user file modified or removed: %s, %v", got, err)
	}
}
