package installer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The genericity gate is a shell script wired into CI, the pre-commit hook and
// the project's check recipe; nothing else proves it actually fires. These cases
// build a throwaway git repo shaped like the shipped bundle — the script anchors
// itself with `git rev-parse --show-toplevel`, so the fixture must be a real repo
// — and assert the exit code for each rule. Every case runs the script from a
// subdirectory, which is the regression the anchoring exists for: before it, a
// run from anywhere but the repo root scanned nothing and still exited 0.
//
// It lives beside the installer — the package that writes those two trees into
// a consuming project, which is the harm the gate prevents — and deliberately
// NOT under anvil/skills/, because its fixtures spell out the exact strings the
// gate hunts for and would trip anvil.0215's own repo-wide verification grep.
func TestGenericityGate(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", ".github", "check-genericity.sh"))
	if err != nil {
		t.Fatalf("resolve script path: %v", err)
	}

	tests := []struct {
		name    string
		files   map[string]string
		noRoots bool
		want    int
	}{
		{
			name:  "clean bundle passes",
			files: map[string]string{"anvil/skills/probe/SKILL.md": "Read the project's gate command from its CLAUDE.md.\n"},
			want:  0,
		},
		{
			name:  "anvil toolchain command fails",
			files: map[string]string{"anvil/skills/probe/SKILL.md": "Run `just check` before opening the PR.\n"},
			want:  1,
		},
		{
			name:  "anvil source-tree path fails",
			files: map[string]string{"anvil/skills/probe/SKILL.md": "The contract lives in `anvil/agents/anvil-issue-worker.md`.\n"},
			want:  1,
		},
		{
			name:  "just install imperative fails",
			files: map[string]string{"anvil/skills/probe/SKILL.md": "Then `just install` && `anvil install agents`.\n"},
			want:  1,
		},
		{
			name:  "just install inside a cross-ecosystem enumeration passes",
			files: map[string]string{"anvil/skills/probe/SKILL.md": "Common shapes: `make install`, `just install`, `npm run build && npm link`, `cargo install --path .`, `pip install -e .`.\n"},
			want:  0,
		},
		{
			name:  "shipped non-markdown script is in scope",
			files: map[string]string{"anvil/skills/probe/scripts/run.sh": "#!/usr/bin/env bash\ngo test ./...\n"},
			want:  1,
		},
		{
			name: "eval fixtures are out of scope",
			files: map[string]string{
				"anvil/skills/probe/evals/evals.json": `{"transcript": "ran just check and go test ./..."}`,
			},
			want: 0,
		},
		{
			name:    "missing scan roots refuse rather than pass vacuously",
			noRoots: true,
			want:    2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := fixtureRepo(t, tc.files, tc.noRoots)
			cmd := exec.Command("bash", script) //nolint:gosec // script path is a test literal resolved from the repo, not user input
			cmd.Dir = filepath.Join(dir, "sub")
			out, err := cmd.CombinedOutput()
			got := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				got = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("run gate: %v", err)
			}
			if got != tc.want {
				t.Fatalf("exit code = %d, want %d\noutput:\n%s", got, tc.want, out)
			}
		})
	}
}

// fixtureRepo materialises a git repo holding the given files, plus the two scan
// roots (unless noRoots) and the `sub/` directory every case is run from.
func fixtureRepo(t *testing.T, files map[string]string, noRoots bool) string {
	t.Helper()
	dir := t.TempDir()
	init := exec.Command("git", "init", "-q")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	dirs := []string{"sub"}
	if !noRoots {
		dirs = append(dirs, filepath.Join("anvil", "skills"), filepath.Join("anvil", "agents"))
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}
