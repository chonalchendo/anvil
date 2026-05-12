package installer

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func fakeSkillsFS() fstest.MapFS {
	return fstest.MapFS{
		"capturing-inbox/SKILL.md":            {Data: []byte("# capturing-inbox\n")},
		"writing-issue/SKILL.md":              {Data: []byte("# writing-issue\n")},
		"writing-issue/references/heavy.md":   {Data: []byte("heavy reference\n")},
	}
}

func TestInstallSkills_Symlink(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")

	changed, err := InstallSkills(fakeSkillsFS(), mat, target, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !changed {
		t.Error("first install should report changed=true")
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target is not a symlink: mode=%v", info.Mode())
	}
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != mat {
		t.Errorf("symlink = %q, want %q", got, mat)
	}

	body, err := os.ReadFile(filepath.Join(target, "capturing-inbox", "SKILL.md"))
	if err != nil {
		t.Fatalf("read via symlink: %v", err)
	}
	if string(body) != "# capturing-inbox\n" {
		t.Errorf("body = %q", body)
	}
}

func TestInstallSkills_SymlinkIdempotent(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallSkills(fakeSkillsFS(), mat, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("second install should report changed=false")
	}
}

func TestInstallSkills_SymlinkReplacesWrongLink(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")
	other := t.TempDir()
	if err := os.Symlink(other, target); err != nil {
		t.Fatal(err)
	}

	changed, err := InstallSkills(fakeSkillsFS(), mat, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("replacing wrong symlink should report changed=true")
	}
	got, _ := os.Readlink(target)
	if got != mat {
		t.Errorf("symlink = %q, want %q", got, mat)
	}
}

func TestInstallSkills_SymlinkRefusesNonSymlinkTarget(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "user.md"), []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err == nil {
		t.Fatal("expected error refusing to clobber non-symlink target")
	}
	if _, err := os.Stat(filepath.Join(target, "user.md")); err != nil {
		t.Errorf("user data was clobbered: %v", err)
	}
}

func TestInstallSkills_Copy(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")

	changed, err := InstallSkills(fakeSkillsFS(), mat, target, true)
	if err != nil {
		t.Fatalf("install --copy: %v", err)
	}
	if !changed {
		t.Error("copy install should report changed=true")
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("target should be a directory, not a symlink, in --copy mode")
	}
	body, err := os.ReadFile(filepath.Join(target, "writing-issue", "references", "heavy.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "heavy reference\n" {
		t.Errorf("body = %q", body)
	}
}

func TestRemoveSkills_Symlink(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}
	changed, err := RemoveSkills(target)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("remove should report changed=true")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("target should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mat, "capturing-inbox", "SKILL.md")); err != nil {
		t.Errorf("materialised dir should be preserved: %v", err)
	}
}

func TestRemoveSkills_CopiedDir(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "anvil")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, true); err != nil {
		t.Fatal(err)
	}
	changed, err := RemoveSkills(target)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("remove should report changed=true")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target should be gone: %v", err)
	}
}

func TestRemoveSkills_Missing(t *testing.T) {
	target := filepath.Join(t.TempDir(), "does-not-exist")
	changed, err := RemoveSkills(target)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("missing target should report changed=false")
	}
}
