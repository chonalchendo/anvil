package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

// vaultGitIgnore lists paths that are vault-local and must not be committed.
// vault.db is a regeneratable index; .obsidian/ tracks per-machine editor state.
const vaultGitIgnore = ".anvil/vault.db\n.obsidian/\n"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Scaffold an Anvil vault",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var v *core.Vault
			if len(args) == 1 {
				v = &core.Vault{Root: args[0]}
			} else {
				rv, err := core.ResolveVault()
				if err != nil {
					return err
				}
				v = rv
			}
			if err := v.Scaffold(); err != nil {
				return err
			}
			entries, err := schema.EmbeddedFS.ReadDir(".")
			if err != nil {
				return fmt.Errorf("read embedded schemas: %w", err)
			}
			for _, e := range entries {
				b, err := schema.EmbeddedFS.ReadFile(e.Name())
				if err != nil {
					return fmt.Errorf("read %s: %w", e.Name(), err)
				}
				target := filepath.Join(v.SchemasDir(), e.Name())
				if err := os.WriteFile(target, b, 0o644); err != nil {
					return fmt.Errorf("write %s: %w", target, err)
				}
			}
			if err := gitInitVault(v.Root); err != nil {
				return err
			}
			cmd.Println("vault scaffolded at", v.Root)
			return nil
		},
	}
}

// gitInitVault turns root into a git repository with an initial commit when it
// is not already version-controlled. It is idempotent: a pre-existing .git dir
// is left untouched so that re-running `anvil init` on an existing vault is
// safe.
func gitInitVault(root string) error {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return nil // already a git repo
	}
	gitIgnorePath := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitIgnorePath, []byte(vaultGitIgnore), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	// Strip inherited git env-vars (GIT_DIR, GIT_INDEX_FILE, etc.) that the
	// caller's hook runner may have set; they would redirect git away from root.
	cleanEnv := cleanGitEnv(os.Environ())
	for _, step := range [][]string{
		{"git", "init", "-q"},
		{"git", "add", "."},
		{"git", "-c", "user.email=anvil@localhost", "-c", "user.name=anvil", "commit", "-q", "-m", "chore: anvil vault scaffold"},
	} {
		c := exec.Command(step[0], step[1:]...) //nolint:gosec
		c.Dir = root
		c.Env = cleanEnv
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("git init: %s: %w", out, err)
		}
	}
	return nil
}

// cleanGitEnv returns env with git plumbing variables removed so that
// subprocess git commands resolve their repository from the working directory
// rather than inheriting an ambient GIT_DIR or GIT_INDEX_FILE.
func cleanGitEnv(env []string) []string {
	drop := map[string]bool{
		"GIT_DIR":              true,
		"GIT_WORK_TREE":        true,
		"GIT_INDEX_FILE":       true,
		"GIT_OBJECT_DIRECTORY": true,
		"GIT_COMMON_DIR":       true,
	}
	out := env[:0:len(env)]
	for _, kv := range env {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if !drop[key] {
			out = append(out, kv)
		}
	}
	return out
}
