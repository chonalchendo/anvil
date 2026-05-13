package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newRenameCmd() *cobra.Command {
	var (
		flagTitle string
		flagJSON  bool
	)

	cmd := &cobra.Command{
		Use:   "rename <type> <id> --title <new-title>",
		Short: "Rename a vault artifact, updating its file, frontmatter, and inbound wikilinks",
		Long: `Rename a vault artifact by title.

The new title is slugified using the same rule as create:
  lowercase → ASCII transliterate (NFD) → non-alnum runs to "-" → trim → clip to 60 chars

If the new slug matches the existing slug (i.e. a cosmetic-only change like
capitalisation), the file is not moved — only the title and updated fields are
written. Use ` + "`anvil set <type> <id> title <value>`" + ` for that case if preferred.

Inbound wikilinks are rewritten across the whole vault. A rewrite failure on
one file is reported on stderr and does not abort the rename — the artifact
rename always takes effect first.`,
		Example: `  anvil rename issue anvil.my-old-title --title "My new title"
  anvil rename issue anvil.my-old-title --title "My new title" --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTitle == "" {
				return fmt.Errorf("--title is required")
			}

			t, err := core.ParseType(args[0])
			if err != nil {
				return fmt.Errorf("type: %w", err)
			}
			oldID := args[1]

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			oldPath := filepath.Join(v.Root, t.Dir(), oldID+".md")
			a, err := core.LoadArtifact(oldPath)
			if err != nil {
				if os.IsNotExist(err) {
					return ErrArtifactNotFound
				}
				return fmt.Errorf("loading artifact: %w", err)
			}

			newSlug := newIDSlug(t, oldID, flagTitle)
			if newSlug == "" {
				return fmt.Errorf("new title %q produces an empty slug", flagTitle)
			}
			newID := replaceSlug(t, oldID, newSlug)
			newPath := filepath.Join(v.Root, t.Dir(), newID+".md")

			if newID == oldID {
				// Cosmetic-only change: slug unchanged, just update the title field.
				a.FrontMatter["title"] = flagTitle
				a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
				if err := a.Save(); err != nil {
					return fmt.Errorf("saving artifact: %w", err)
				}
				if err := indexAfterSave(v, a); err != nil {
					return fmt.Errorf("indexing: %w", err)
				}
				return emitRenameResult(cmd, flagJSON, renameResult{
					OldID: oldID, NewID: newID,
					OldPath: oldPath, NewPath: newPath,
					Status: "cosmetic",
				})
			}

			if _, err := os.Stat(newPath); err == nil {
				return fmt.Errorf("target %s already exists; choose a different title", newID)
			}

			// Update frontmatter before writing to new path.
			a.FrontMatter["title"] = flagTitle
			a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
			// Wipe any explicit slug field — the filename is the canonical ID.
			delete(a.FrontMatter, "slug")

			// Write to new path, then remove old file.
			a.Path = newPath
			if err := a.Save(); err != nil {
				return fmt.Errorf("writing new artifact: %w", err)
			}
			if err := os.Remove(oldPath); err != nil {
				// New file exists; try to clean it up before returning the error.
				_ = os.Remove(newPath)
				return fmt.Errorf("removing old artifact: %w", err)
			}

			// Update the index: register new ID, drop old one.
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close()
			if _, err := db.Reindex(v.Root); err != nil {
				return fmt.Errorf("reindexing after rename: %w", err)
			}

			// Rewrite inbound wikilinks across the vault.
			oldWikilink := fmt.Sprintf("[[%s.%s]]", t, oldID)
			newWikilink := fmt.Sprintf("[[%s.%s]]", t, newID)
			rewritten, skipped := rewriteWikilinks(v.Root, oldWikilink, newWikilink, newPath)

			r := renameResult{
				OldID: oldID, NewID: newID,
				OldPath: oldPath, NewPath: newPath,
				LinksRewritten: rewritten, LinksSkipped: skipped,
				Status: "renamed",
			}
			if len(skipped) > 0 {
				for _, s := range skipped {
					cmd.PrintErrf("WARN: could not rewrite wikilink in %s\n", s)
				}
			}
			return emitRenameResult(cmd, flagJSON, r)
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "new title for the artifact (required)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// newIDSlug computes the slug portion of the new ID from the new title.
func newIDSlug(_ core.Type, _ string, newTitle string) string {
	return core.Slugify(newTitle)
}

// replaceSlug replaces the slug part of an existing ID with newSlug.
// For project-scoped types (issue, plan, milestone) the ID is <project>.<slug>.
// For inbox it is <date>-<slug>. For others it is just the slug.
func replaceSlug(t core.Type, oldID, newSlug string) string {
	switch t {
	case core.TypeIssue, core.TypePlan, core.TypeMilestone:
		dot := strings.IndexByte(oldID, '.')
		if dot >= 0 {
			return oldID[:dot+1] + newSlug
		}
	case core.TypeInbox:
		// ID format: <date>-<slug>; date portion is YYYY-MM-DD (10 chars + dash = 11).
		if len(oldID) > 11 && oldID[10] == '-' {
			return oldID[:11] + newSlug
		}
	case core.TypeDecision:
		// ID format: <topic>.<ordinal>-<slug>; preserve up through the ordinal dash.
		dot := strings.IndexByte(oldID, '.')
		if dot >= 0 {
			rest := oldID[dot+1:]
			dash := strings.IndexByte(rest, '-')
			if dash >= 0 {
				return oldID[:dot+1] + rest[:dash+1] + newSlug
			}
		}
	}
	// Thread, learning, sweep, session, product-design, system-design: id == slug.
	return newSlug
}

// rewriteWikilinks walks vaultRoot and replaces every occurrence of oldLink
// with newLink in .md files (skipping skipPath, the just-renamed file itself).
// Returns lists of rewritten and failed file paths.
func rewriteWikilinks(vaultRoot, oldLink, newLink, skipPath string) (rewritten, skipped []string) {
	_ = filepath.WalkDir(vaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		if path == skipPath {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			skipped = append(skipped, path)
			return nil
		}
		content := string(b)
		if !strings.Contains(content, oldLink) {
			return nil
		}
		updated := strings.ReplaceAll(content, oldLink, newLink)
		if werr := os.WriteFile(path, []byte(updated), 0o644); werr != nil {
			skipped = append(skipped, path)
			return nil
		}
		rewritten = append(rewritten, path)
		return nil
	})
	return rewritten, skipped
}

type renameResult struct {
	OldID          string   `json:"old_id"`
	NewID          string   `json:"new_id"`
	OldPath        string   `json:"old_path"`
	NewPath        string   `json:"new_path"`
	LinksRewritten []string `json:"links_rewritten,omitempty"`
	LinksSkipped   []string `json:"links_skipped,omitempty"`
	Status         string   `json:"status"`
}

func emitRenameResult(cmd *cobra.Command, asJSON bool, r renameResult) error {
	if asJSON {
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	switch r.Status {
	case "cosmetic":
		fmt.Fprintf(cmd.OutOrStdout(), "%s: title updated (slug unchanged)\n", r.OldID)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", r.OldID, r.NewID)
		if len(r.LinksRewritten) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  rewritten links in %d file(s)\n", len(r.LinksRewritten))
		}
	}
	return nil
}
