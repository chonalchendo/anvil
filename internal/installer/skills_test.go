package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func fakeSkillsFS() fstest.MapFS {
	return fstest.MapFS{
		"capturing-inbox/SKILL.md":          {Data: []byte("# capturing-inbox\n")},
		"writing-issue/SKILL.md":            {Data: []byte("# writing-issue\n")},
		"writing-issue/references/heavy.md": {Data: []byte("heavy reference\n")},
	}
}

func TestInstallSkills_FlatPerSkillSymlinks(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")

	changed, err := InstallSkills(fakeSkillsFS(), mat, target, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !changed {
		t.Error("first install should report changed=true")
	}

	for _, name := range []string{"capturing-inbox", "writing-issue"} {
		child := filepath.Join(target, name)
		info, err := os.Lstat(child)
		if err != nil {
			t.Fatalf("lstat %s: %v", child, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s should be a symlink, mode=%v", child, info.Mode())
		}
		want := filepath.Join(mat, name)
		got, _ := os.Readlink(child)
		if got != want {
			t.Errorf("symlink target = %q, want %q", got, want)
		}
	}

	body, err := os.ReadFile(filepath.Join(target, "capturing-inbox", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md via symlink: %v", err)
	}
	if string(body) != "# capturing-inbox\n" {
		t.Errorf("body = %q", body)
	}
}

func TestInstallSkills_CleansUpLegacyNestedInstall(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")

	legacy := filepath.Join(target, "anvil")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, ".anvil-skills-hash"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy target/anvil should be removed: %v", err)
	}
}

func TestInstallSkills_PreservesForeignAnvilDir(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")

	if err := os.MkdirAll(filepath.Join(target, "anvil"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "anvil", "user.md"), []byte("user content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, "anvil", "user.md")); err != nil {
		t.Errorf("foreign anvil/ was clobbered: %v", err)
	}
}

func TestInstallSkills_IdempotentFlat(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
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

func TestInstallSkills_ReplacesWrongSymlink(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(target, "capturing-inbox")); err != nil {
		t.Fatal(err)
	}

	changed, err := InstallSkills(fakeSkillsFS(), mat, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("replacing wrong symlink should report changed=true")
	}
	got, _ := os.Readlink(filepath.Join(target, "capturing-inbox"))
	want := filepath.Join(mat, "capturing-inbox")
	if got != want {
		t.Errorf("symlink = %q, want %q", got, want)
	}
}

func TestInstallSkills_RefusesNonSymlinkAtSkillName(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if err := os.MkdirAll(filepath.Join(target, "capturing-inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "capturing-inbox", "user.md"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallSkills(fakeSkillsFS(), mat, target, false)
	if err == nil {
		t.Fatal("expected error refusing to clobber non-symlink at shipped-name path")
	}
	msg := err.Error()
	if !strings.Contains(msg, "anvil install skills --force") {
		t.Errorf("symlink refusal must name --force command verbatim; got: %s", msg)
	}
	if !strings.Contains(msg, "rm -rf") {
		t.Errorf("symlink refusal must name rm -rf escape; got: %s", msg)
	}
	if _, err := os.Stat(filepath.Join(target, "capturing-inbox", "user.md")); err != nil {
		t.Errorf("user data was clobbered: %v", err)
	}
}

func TestInstallSkills_RefusesNonAnvilDirNamesForceCommand(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	foreign := filepath.Join(target, "capturing-inbox")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "user.md"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallSkills(fakeSkillsFS(), mat, target, true)
	if err == nil {
		t.Fatal("expected refusal error on foreign non-anvil dir in --copy mode")
	}
	msg := err.Error()
	if !strings.Contains(msg, "anvil install skills --force") {
		t.Errorf("copy refusal must name --force command verbatim; got: %s", msg)
	}
	if !strings.Contains(msg, "rm -rf") {
		t.Errorf("copy refusal must name rm -rf escape; got: %s", msg)
	}
	if !strings.Contains(msg, "refusing to overwrite") {
		t.Errorf("copy refusal must keep 'refusing to overwrite' prefix (T2 depends on it); got: %s", msg)
	}
	if _, err := os.Stat(filepath.Join(foreign, "user.md")); err != nil {
		t.Errorf("foreign data was clobbered: %v", err)
	}
}

func TestInstallSkills_FlatCopyMode(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")

	changed, err := InstallSkills(fakeSkillsFS(), mat, target, true)
	if err != nil {
		t.Fatalf("install --copy: %v", err)
	}
	if !changed {
		t.Error("copy install should report changed=true")
	}
	for _, name := range []string{"capturing-inbox", "writing-issue"} {
		info, err := os.Lstat(filepath.Join(target, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s should be a dir, not a symlink, in --copy mode", name)
		}
	}
	body, err := os.ReadFile(filepath.Join(target, "writing-issue", "references", "heavy.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "heavy reference\n" {
		t.Errorf("body = %q", body)
	}
}

func TestRemoveSkills_FlatSymlinks(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}

	changed, err := RemoveSkills(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("remove should report changed=true")
	}
	for _, name := range []string{"capturing-inbox", "writing-issue"} {
		if _, err := os.Lstat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Errorf("%s should be gone: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(mat, "capturing-inbox", "SKILL.md")); err != nil {
		t.Errorf("materialised dir should be preserved: %v", err)
	}
}

func TestRemoveSkills_FlatCopied(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, true); err != nil {
		t.Fatal(err)
	}

	changed, err := RemoveSkills(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("remove should report changed=true")
	}
	for _, name := range []string{"capturing-inbox", "writing-issue"} {
		if _, err := os.Stat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Errorf("%s should be gone: %v", name, err)
		}
	}
}

func TestRemoveSkills_PreservesForeignSibling(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}

	foreign := filepath.Join(target, "other-vendor-skill")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RemoveSkills(fakeSkillsFS(), mat, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(foreign, "SKILL.md")); err != nil {
		t.Errorf("foreign sibling was removed: %v", err)
	}
}

func TestRemoveSkills_RemovesLegacyNested(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	legacy := filepath.Join(target, "anvil")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, ".anvil-skills-hash"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RemoveSkills(fakeSkillsFS(), mat, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy nested install should be removed: %v", err)
	}
}

func TestRemoveSkills_Missing(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "does-not-exist")
	changed, err := RemoveSkills(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("missing target should report changed=false")
	}
}

func TestInstallSkills_WritesHashFile(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(mat, skillsHashFile))
	if err != nil {
		t.Fatalf("read hash file: %v", err)
	}
	if len(data) == 0 {
		t.Error("hash file is empty")
	}
}

func TestRefreshSkillsIfStale_NoOpWhenAbsent(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	refreshed, err := RefreshSkillsIfStale(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed {
		t.Error("absent materialiseDir should not be refreshed")
	}
	if _, err := os.Stat(mat); !os.IsNotExist(err) {
		t.Errorf("materialiseDir should remain absent: %v", err)
	}
}

func TestRefreshSkillsIfStale_NoOpWhenFresh(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshSkillsIfStale(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Error("fresh install should not refresh")
	}
}

func TestRefreshSkillsIfStale_RefreshesWhenContentDrifts(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}

	skill := filepath.Join(mat, "capturing-inbox", "SKILL.md")
	if err := os.WriteFile(skill, []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mat, skillsHashFile), []byte("stale-hash"), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshSkillsIfStale(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !refreshed {
		t.Fatal("drifted materialiseDir should refresh")
	}

	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# capturing-inbox\n" {
		t.Errorf("after refresh body = %q, want canonical content", body)
	}
}

func TestRefreshSkillsIfStale_RefreshesWhenHashFileMissing(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(mat, skillsHashFile)); err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshSkillsIfStale(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Error("missing hash file should trigger refresh")
	}
	if _, err := os.Stat(filepath.Join(mat, skillsHashFile)); err != nil {
		t.Errorf("hash file should be rewritten: %v", err)
	}
}

func TestRefreshSkillsIfStale_PreservesCopyMode(t *testing.T) {
	mat := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "claude-skills")
	if _, err := InstallSkills(fakeSkillsFS(), mat, target, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mat, skillsHashFile), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshSkillsIfStale(fakeSkillsFS(), mat, target)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("stale copy install should refresh")
	}
	info, err := os.Lstat(filepath.Join(target, "capturing-inbox"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("per-skill child should remain a directory after refresh in copy mode")
	}
	body, err := os.ReadFile(filepath.Join(target, "capturing-inbox", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# capturing-inbox\n" {
		t.Errorf("body = %q", body)
	}
}
