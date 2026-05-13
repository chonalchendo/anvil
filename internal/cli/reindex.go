package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newReindexCmd() *cobra.Command {
	var asJSON bool
	var pruneStubs bool
	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild .anvil/vault.db from the vault on disk",
		Long: `Rebuild .anvil/vault.db from the vault on disk.

Detects stray "<type>.*.md" files at the vault root (typically created by
Obsidian wikilink clicks that resolve to missing artifacts) and emits a WARN
line per stub on stderr. Stubs are not added to the index — they have no
frontmatter — but they pollute the vault.

With --prune-stubs, 0-byte stubs are deleted. Non-zero stubs are reported but
never deleted: they may hold user-authored content the wikilink-creation flow
populated after the fact, and removing them would be destructive.`,
		Example: `  anvil reindex
  anvil reindex --json
  anvil reindex --prune-stubs`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return err
			}
			defer db.Close()
			stats, err := db.Reindex(v.Root)
			if err != nil {
				return err
			}

			stubs, err := core.FindStubs(v.Root)
			if err != nil {
				return fmt.Errorf("scanning for stubs: %w", err)
			}
			pruned, kept := handleStubs(cmd, stubs, pruneStubs)

			if asJSON {
				payload := map[string]any{
					"artifacts":   stats.Artifacts,
					"links":       stats.Links,
					"duration_ms": stats.DurationMS,
					"stubs":       stubFilenames(stubs),
					"pruned":      stubFilenames(pruned),
				}
				b, _ := json.Marshal(payload)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reindexed: %d artifacts, %d links (%dms)\n", stats.Artifacts, stats.Links, stats.DurationMS)
			if pruneStubs && len(pruned) > 0 {
				cmd.PrintErrf("pruned %d 0-byte stub(s); %d non-empty stub(s) kept\n", len(pruned), len(kept))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&pruneStubs, "prune-stubs", false, "delete 0-byte stray <type>.*.md files at vault root (non-empty stubs are kept and warned about)")
	return cmd
}

// handleStubs warns on every detected stub and, when prune is true, deletes
// the 0-byte ones. Returns (pruned, kept) for caller reporting. Non-zero stubs
// are never deleted — they may hold user content.
func handleStubs(cmd *cobra.Command, stubs []core.Stub, prune bool) (pruned, kept []core.Stub) {
	for _, s := range stubs {
		switch {
		case s.Size == 0 && prune:
			if err := os.Remove(s.Path); err != nil {
				cmd.PrintErrf("WARN: stub %s: prune failed: %v\n", s.Path, err)
				kept = append(kept, s)
				continue
			}
			cmd.PrintErrf("pruned: %s (0 bytes)\n", s.Path)
			pruned = append(pruned, s)
		case s.Size == 0:
			cmd.PrintErrf("WARN: 0-byte stub at vault root: %s (run `anvil reindex --prune-stubs` to delete)\n", s.Path)
			kept = append(kept, s)
		default:
			cmd.PrintErrf("WARN: stray artifact-named file at vault root: %s (%d bytes; move into the canonical <NN>-<type>/ dir)\n", s.Path, s.Size)
			kept = append(kept, s)
		}
	}
	return pruned, kept
}

func stubFilenames(stubs []core.Stub) []string {
	out := make([]string, 0, len(stubs))
	for _, s := range stubs {
		out = append(out, s.Path)
	}
	return out
}
