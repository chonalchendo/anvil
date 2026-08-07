package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Name kept despite the inverted assertion: the issue's Direct predicate
// selects tests via `-run 'Refresh'`, so renaming would silently select zero.
//
// TestRoot_NeverRefreshesStaleSkills verifies that a non-install verb never
// rewrites the installed skills bundle, even when the recorded hash has
// drifted from the binary's embedded skills. The bundle changes only under
// an explicit `anvil install skills` — see issue.anvil.0242.
func TestRoot_NeverRefreshesStaleSkills(t *testing.T) {
	skillsRoot := t.TempDir()
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)

	setupVault(t)

	if _, _, err := runCmd(t, newRootCmd(), "install", "skills"); err != nil {
		t.Fatalf("install skills: %v", err)
	}

	hashPath := filepath.Join(skillsRoot, ".anvil-skills-hash")
	if err := os.WriteFile(hashPath, []byte("stale"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatalf("corrupt hash: %v", err)
	}

	// Write a sentinel directly into a materialised skill body. Checking the
	// recorded hash alone would pass even if a rewrite clobbered skill
	// content but skipped re-stamping the hash — the sentinel catches that.
	skillFile := filepath.Join(skillsRoot, "completing-issue", "SKILL.md")
	const sentinel = "\n<!-- sentinel-0242 -->\n"
	orig, err := os.ReadFile(skillFile) //nolint:gosec // path is test-controlled
	if err != nil {
		t.Fatalf("read materialised skill: %v", err)
	}
	if err := os.WriteFile(skillFile, append(orig, []byte(sentinel)...), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatalf("stamp sentinel: %v", err)
	}

	_, errOut, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("where: %v\nstderr: %s", err, errOut)
	}
	if strings.Contains(errOut, "refreshed stale skills bundle") {
		t.Errorf("non-install verb must not emit a refresh notice, got:\n%s", errOut)
	}

	data, err := os.ReadFile(hashPath) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "stale" {
		t.Error("non-install verb rewrote the recorded hash; only `anvil install skills` may")
	}

	after, err := os.ReadFile(skillFile) //nolint:gosec // path is test-controlled
	if err != nil {
		t.Fatalf("read materialised skill after: %v", err)
	}
	if !strings.Contains(string(after), sentinel) {
		t.Error("non-install verb rewrote materialised skill content; only `anvil install skills` may")
	}
}

// TestRoot_SkipsRefreshWhenSkillsAbsent confirms a non-install verb never
// creates the skills materialise dir when the user never ran
// `anvil install skills` — no notice, no mkdir.
func TestRoot_SkipsRefreshWhenSkillsAbsent(t *testing.T) {
	skillsRoot := filepath.Join(t.TempDir(), "never-installed")
	claudeRoot := t.TempDir()
	t.Setenv("ANVIL_SKILLS_DIR", skillsRoot)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeRoot)
	setupVault(t)

	_, errOut, err := runCmd(t, newRootCmd(), "where")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	if strings.Contains(errOut, "refreshed") {
		t.Errorf("unexpected refresh notice: %s", errOut)
	}
	if _, err := os.Stat(skillsRoot); !os.IsNotExist(err) {
		t.Errorf("non-install verb should not have created %s: %v", skillsRoot, err)
	}
}
