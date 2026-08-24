package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newRenumberCmd() *cobra.Command {
	var (
		flagTo   int
		flagJSON bool
	)

	cmd := &cobra.Command{
		Use:   "renumber issue <id> [--to N]",
		Short: "Move a numbered issue onto a free ordinal, rewriting inbound wikilinks",
		Long: `Move a numbered issue onto another ordinal, keeping its project and slug.

The repair for a duplicate ordinal (` + "`anvil doctor`" + ` kind duplicate-ordinal):
a vault clone that was behind the origin re-mints ordinals the origin had
already used, and git merges the two files without conflict. Pass the full
id — ordinal shorthand refuses while it is ambiguous. --to claims a specific
free ordinal; otherwise the next free one is taken, through the same
reservation a concurrent create honours.

Inbound wikilinks are rewritten across the whole vault; a rewrite failure on
one file is reported on stderr and does not undo the move.`,
		Example: `  anvil renumber issue issue.mentat.0452.the-later-one
  anvil renumber issue issue.mentat.0452.the-later-one --to 470 --json`,
		Args: namedArgs("renumber issue <id>", []string{"<type>", "<id>"}, 2, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != string(core.TypeIssue) {
				return fmt.Errorf("%w: renumber applies to issue only, got %q", ErrUnsupportedForType, args[0])
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			oldID, oldPath, err := core.ResolveArtifact(v, core.TypeIssue, args[1])
			if err != nil {
				return err
			}
			a, err := core.LoadArtifact(oldPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: %s", ErrArtifactNotFound, oldID)
				}
				return fmt.Errorf("loading artifact: %w", err)
			}
			parts := strings.SplitN(core.BareID(core.TypeIssue, oldID), ".", 3)
			if len(parts) != 3 || !core.IsOrdinalOnly(parts[1]) {
				return fmt.Errorf("%s carries no ordinal to renumber", oldID)
			}
			project, slug := parts[0], parts[2]

			newID, newPath, release, err := core.ReserveIssueOrdinal(v, project, slug, flagTo)
			if err != nil {
				return err
			}
			defer release()

			a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
			a.Path = newPath
			content, err := a.Marshal()
			if err != nil {
				return fmt.Errorf("marshalling artifact: %w", err)
			}
			if err := atomicSwap(oldPath, newPath, content); err != nil {
				return fmt.Errorf("atomic rename: %w", err)
			}
			rewritten, skipped := sweepWikilinks(cmd, v, core.TypeIssue, oldID, newID, newPath)
			// After the sweep, so the swept files land inside the stamp too.
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable
			if _, err := db.Reindex(v.Root); err != nil {
				cmd.PrintErrf("WARN: reindex after renumber failed: %v\n", err)
			}

			r := renumberResult{
				ID: newID, OldID: oldID,
				OldPath: oldPath, NewPath: newPath,
				LinksRewritten: rewritten, LinksSkipped: skipped,
			}
			if flagJSON {
				b, _ := json.Marshal(r)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", oldID, newID)
			if len(rewritten) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  rewritten links in %d file(s)\n", len(rewritten))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flagTo, "to", 0, "claim this ordinal instead of the next free one")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	return cmd
}

type renumberResult struct {
	ID             string   `json:"id"`
	OldID          string   `json:"old_id"`
	OldPath        string   `json:"old_path"`
	NewPath        string   `json:"new_path"`
	LinksRewritten []string `json:"links_rewritten"`
	LinksSkipped   []string `json:"links_skipped"`
}
