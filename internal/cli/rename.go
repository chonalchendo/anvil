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
		flagSlug  string
		flagJSON  bool
	)

	cmd := &cobra.Command{
		Use:   "rename <type> <id> --title <new-title>",
		Short: "Rename a vault artifact, updating its file, frontmatter, and inbound wikilinks",
		Long: `Rename a vault artifact by title.

The new title is slugified using the same rule as create:
  lowercase → ASCII transliterate (NFD) → non-alnum runs to "-" → trim → clip to 60 chars

Use --slug to set the new id's slug explicitly instead of deriving it from
--title — e.g. when the title-derived slug would collide, or the desired id
diverges from the title.

If the new slug matches the existing slug (i.e. a cosmetic-only change like
capitalisation), the file is not moved — only the title and updated fields are
written. Use ` + "`anvil set <type> <id> title <value>`" + ` for that case if preferred.

Inbound wikilinks are rewritten across the whole vault. A rewrite failure on
one file is reported on stderr and does not abort the rename — the artifact
rename always takes effect first.`,
		Example: `  anvil rename issue anvil.my-old-title --title "My new title"
  anvil rename issue anvil.my-old-title --title "My new title" --json
  anvil rename issue anvil.my-old-title --title "My new title" --slug custom-slug`,
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

			// ResolveArtifact accepts either filename shape (canonical
			// `issue.foo.x.md` or bare back-catalogue `foo.x.md`) and either
			// id shape as the argument, same as every other write verb.
			oldID, oldPath := core.ResolveArtifact(v, t, oldID)
			a, err := core.LoadArtifact(oldPath)
			if err != nil {
				if os.IsNotExist(err) {
					return ErrArtifactNotFound
				}
				return fmt.Errorf("loading artifact: %w", err)
			}

			newSlug := flagSlug
			if newSlug != "" {
				if err := core.ValidateSlug(newSlug); err != nil {
					return fmt.Errorf("--slug: %w", err)
				}
			} else {
				newSlug = core.Slugify(flagTitle)
				if newSlug == "" {
					return fmt.Errorf("new title %q produces an empty slug", flagTitle)
				}
			}
			newID, err := replaceSlug(t, oldID, newSlug, flagSlug != "")
			if err != nil {
				return err
			}
			newPath := filepath.Join(v.Root, t.Dir(), newID+".md")

			if newID == oldID {
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

			// Probe both filename shapes: a bare back-catalogue
			// `foo.new-title.md` names the same artifact as canonical
			// `issue.foo.new-title.md`, so either blocks the rename.
			_, existingPath := core.ResolveArtifact(v, t, newID)
			if _, err := os.Stat(existingPath); err == nil {
				return fmt.Errorf("target %s already exists; choose a different --title or --slug", newID)
			}

			a.FrontMatter["title"] = flagTitle
			a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
			// Wipe any explicit slug field — the filename is the canonical ID.
			delete(a.FrontMatter, "slug")

			a.Path = newPath
			content, err := a.Marshal()
			if err != nil {
				return fmt.Errorf("marshalling artifact: %w", err)
			}
			if err := atomicSwap(oldPath, newPath, content); err != nil {
				return fmt.Errorf("atomic rename: %w", err)
			}

			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable
			if _, err := db.Reindex(v.Root); err != nil {
				cmd.PrintErrf("WARN: reindex after rename failed: %v\n", err)
			}

			oldWikilink := "[[" + core.WikilinkTarget(t, oldID) + "]]"
			newWikilink := "[[" + core.WikilinkTarget(t, newID) + "]]"

			rewritten := make([]string, 0)
			skipped := make([]string, 0)
			_ = filepath.WalkDir(v.Root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
					return nil
				}
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".md") {
					return nil
				}
				if path == newPath {
					return nil
				}
				b, rerr := os.ReadFile(path) //nolint:gosec // G304: path is a descendant of v.Root yielded by filepath.WalkDir
				if rerr != nil {
					skipped = append(skipped, path)
					return nil //nolint:nilerr // best-effort rewrite; unreadable files surface via skipped[]
				}
				content := string(b)
				if !strings.Contains(content, oldWikilink) {
					return nil
				}
				fi, statErr := os.Stat(path)
				mode := os.FileMode(0o644)
				if statErr == nil {
					mode = fi.Mode().Perm()
				}
				updated := strings.ReplaceAll(content, oldWikilink, newWikilink)
				if werr := os.WriteFile(path, []byte(updated), mode); werr != nil { //nolint:gosec // G304: path is a descendant of v.Root yielded by filepath.WalkDir; mode preserved from the existing file
					skipped = append(skipped, path)
					return nil //nolint:nilerr // best-effort rewrite; unwritable files surface via skipped[]
				}
				rewritten = append(rewritten, path)
				return nil
			})

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
	cmd.Flags().StringVar(&flagSlug, "slug", "", "override the title-derived slug (must match ^[a-z0-9][a-z0-9-]*$)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// replaceSlug rebuilds an artifact id around newSlug, preserving everything
// before the slug segment: issue/plan/milestone/contract/convention keep the
// canonical type prefix, the project, and (for numbered issues) the ordinal;
// inbox keeps its date prefix; decision keeps topic + MADR ordinal. Slugs
// never contain dots, so the slug is always the last dot-segment of the bare
// id. explicitSlug is true only when the caller passed --slug rather than
// deriving newSlug from --title; it governs the design-type branch, where a
// title-only rename of a singleton must not touch the id (it has no slug
// component) but an explicit --slug does.
func replaceSlug(t core.Type, oldID, newSlug string, explicitSlug bool) (string, error) {
	switch t {
	case core.TypeIssue, core.TypePlan, core.TypeMilestone, core.TypeContract:
		bare := core.BareID(t, oldID)
		if dot := strings.LastIndexByte(bare, '.'); dot >= 0 {
			return core.CanonicalID(t, bare[:dot+1]+newSlug), nil
		}
	case core.TypeConvention:
		return core.CanonicalID(t, newSlug), nil
	case core.TypeSystemDesign:
		// id is bare `<project>` (singleton — no slug component) or
		// `<project>.<slug>` (named shard). An explicit --slug turns a
		// singleton into a named shard; a title-only rename never touches
		// the id.
		if project, _, ok := strings.Cut(oldID, "."); ok {
			return project + "." + newSlug, nil
		}
		if explicitSlug {
			return oldID + "." + newSlug, nil
		}
		return oldID, nil
	case core.TypeProductDesign:
		// id is always <project> — no slug component, ever.
		if explicitSlug {
			return "", fmt.Errorf("--slug is not supported for %s: id is always the project (%s)", t, oldID)
		}
		return oldID, nil
	case core.TypeInbox:
		if len(oldID) > 11 && oldID[10] == '-' {
			return oldID[:11] + newSlug, nil
		}
	case core.TypeDecision, core.TypeThread:
		// Slice rather than re-format so a legacy unpadded ordinal survives
		// verbatim; a bare-slug back-catalogue thread has no topic/ordinal to
		// preserve and falls through to the plain new slug.
		if _, _, slug, ok := core.SplitTopicOrdinal(oldID); ok {
			return strings.TrimSuffix(oldID, slug) + newSlug, nil
		}
	}
	return newSlug, nil
}

type renameResult struct {
	OldID          string   `json:"old_id"`
	NewID          string   `json:"new_id"`
	OldPath        string   `json:"old_path"`
	NewPath        string   `json:"new_path"`
	LinksRewritten []string `json:"links_rewritten"`
	LinksSkipped   []string `json:"links_skipped"`
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
