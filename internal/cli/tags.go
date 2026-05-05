package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newTagsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Inspect tag usage across the vault",
	}
	cmd.AddCommand(newTagsListCmd())
	return cmd
}

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func newTagsListCmd() *cobra.Command {
	var (
		flagType   string
		flagPrefix string
		flagJSON   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tags used across vault artifacts with usage counts",
		Long: `Walks the vault, collects tags from every artifact's frontmatter,
deduplicates, and returns counts. Filter by artifact type with --type and
by tag prefix (e.g. domain/) with --prefix. Use --json for machine output.`,
		Example: `  anvil tags list
  anvil tags list --type learning --json
  anvil tags list --prefix domain/`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			var typeFilter *core.Type
			if flagType != "" {
				t, err := core.ParseType(flagType)
				if err != nil {
					return err
				}
				typeFilter = &t
			}

			counts := map[string]int{}
			types := core.AllTypes
			if typeFilter != nil {
				types = []core.Type{*typeFilter}
			}
			seenDirs := map[string]struct{}{}
			for _, t := range types {
				dir := filepath.Join(v.Root, t.Dir())
				if _, dup := seenDirs[dir]; dup {
					continue // product-design + system-design share 05-projects
				}
				seenDirs[dir] = struct{}{}
				if err := walkTags(dir, typeFilter, counts); err != nil {
					return err
				}
			}

			out := make([]tagCount, 0, len(counts))
			for tag, n := range counts {
				if flagPrefix != "" && !strings.HasPrefix(tag, flagPrefix) {
					continue
				}
				out = append(out, tagCount{Tag: tag, Count: n})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Count != out[j].Count {
					return out[i].Count > out[j].Count
				}
				return out[i].Tag < out[j].Tag
			})

			if flagJSON {
				b, err := json.Marshal(out)
				if err != nil {
					return err
				}
				cmd.Println(string(b))
				return nil
			}
			for _, tc := range out {
				cmd.Printf("%d\t%s\n", tc.Count, tc.Tag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "filter to one artifact type (e.g. learning)")
	cmd.Flags().StringVar(&flagPrefix, "prefix", "", "filter tags by prefix (e.g. domain/)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON [{tag, count}, ...]")
	return cmd
}

// walkTags accumulates tag counts under dir. If typeFilter is set, only
// artifacts whose frontmatter `type` equals *typeFilter are counted —
// this catches the 05-projects directory which holds two singleton types.
func walkTags(dir string, typeFilter *core.Type, counts map[string]int) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // missing type dir is fine in fresh vaults
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		a, err := core.LoadArtifact(path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", path, err)
		}
		if typeFilter != nil {
			tStr, _ := a.FrontMatter["type"].(string)
			if tStr != string(*typeFilter) {
				return nil
			}
		}
		raw, ok := a.FrontMatter["tags"].([]any)
		if !ok {
			return nil
		}
		for _, item := range raw {
			s, ok := item.(string)
			if !ok || s == "" {
				continue
			}
			counts[s]++
		}
		return nil
	})
}
