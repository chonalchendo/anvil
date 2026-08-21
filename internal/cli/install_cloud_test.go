package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// writeClone materialises a checkout whose git remote carries origin, so
// discovery sees the same shape it sees in a cloud session's clone.
func writeClone(t *testing.T, dir, origin string) string {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := "[remote \"origin\"]\n\turl = " + origin + "\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProvisionCloudSessionInertOutsideCloud(t *testing.T) {
	isolateRootEnv(t)
	t.Setenv(cloudSessionEnv, "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := provisionCloudSession()
	if err != nil {
		t.Fatalf("provisionCloudSession: %v", err)
	}
	if report.Provisioned {
		t.Errorf("provisioned = true outside a cloud session, want false")
	}
	if report.Reason != "not a cloud session" {
		t.Errorf("reason = %q, want %q", report.Reason, "not a cloud session")
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("home mutated outside a cloud session: %v", entries)
	}
}

func TestDiscoverVaultCloneMatchesOnRemote(t *testing.T) {
	home := t.TempDir()
	writeClone(t, filepath.Join(home, "some-app"), "https://github.com/chonalchendo/some-app.git")
	want := writeClone(t, filepath.Join(home, "checkout"), "https://github.com/chonalchendo/anvil-vault.git")

	got, err := discoverVaultClone(home)
	if err != nil {
		t.Fatalf("discoverVaultClone: %v", err)
	}
	if got != want {
		t.Errorf("discoverVaultClone = %q, want %q", got, want)
	}
}

func TestDiscoverVaultCloneFindsNestedClone(t *testing.T) {
	home := t.TempDir()
	want := writeClone(t, filepath.Join(home, "chonalchendo", "anvil-vault"), "git@github.com:chonalchendo/anvil-vault.git")

	got, err := discoverVaultClone(home)
	if err != nil {
		t.Fatalf("discoverVaultClone: %v", err)
	}
	if got != want {
		t.Errorf("discoverVaultClone = %q, want %q", got, want)
	}
}

func TestDiscoverVaultCloneReportsMissingVault(t *testing.T) {
	home := t.TempDir()
	writeClone(t, filepath.Join(home, "some-app"), "https://github.com/chonalchendo/some-app.git")

	if _, err := discoverVaultClone(home); err == nil {
		t.Fatal("discoverVaultClone succeeded with no vault clone attached, want error")
	}
}

func TestBindCloudVaultPrefersExplicitEnv(t *testing.T) {
	isolateRootEnv(t)
	t.Setenv("ANVIL_VAULT", "/explicit/vault")
	t.Setenv("HOME", t.TempDir())

	got, err := bindCloudVault()
	if err != nil {
		t.Fatalf("bindCloudVault: %v", err)
	}
	if got != "/explicit/vault" {
		t.Errorf("bindCloudVault = %q, want the explicit ANVIL_VAULT", got)
	}
}

func TestBindCloudVaultLinksAttachedClone(t *testing.T) {
	isolateRootEnv(t)
	t.Setenv("ANVIL_VAULT", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	clone := writeClone(t, filepath.Join(home, "checkout"), "https://github.com/chonalchendo/anvil-vault.git")

	got, err := bindCloudVault()
	if err != nil {
		t.Fatalf("bindCloudVault: %v", err)
	}
	want := filepath.Join(home, "anvil-vault")
	if got != want {
		t.Errorf("bindCloudVault = %q, want %q", got, want)
	}
	resolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("resolving linked vault: %v", err)
	}
	wantResolved, err := filepath.EvalSymlinks(clone)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != wantResolved {
		t.Errorf("linked vault resolves to %q, want %q", resolved, wantResolved)
	}
}
