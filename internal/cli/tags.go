package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
)

func newTagsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Inspect tag usage across the vault",
	}
	cmd.AddCommand(newTagsListCmd())
	return cmd
}

type tagRow struct {
	Tag     string `json:"tag"`
	Count   int    `json:"count"`
	Defined bool   `json:"defined"`
}

func newTagsListCmd() *cobra.Command {
	var (
		flagType   string
		flagPrefix string
		flagSource string
		flagLimit  int
		flagJSON   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List vault tags (used, defined, or both)",
		Long: `--source used (default): tag counts observed in artifact frontmatter.
--source defined: vocabulary entries from _meta/glossary.md (no counts).
--source all: union with {tag, count, defined}; count is 0 if defined-only.`,
		Example: `  anvil tags list
  anvil tags list --source defined --json
  anvil tags list --source all --prefix domain/ --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			validSources := []string{"used", "defined", "all"}
			ok := false
			for _, s := range validSources {
				if flagSource == s {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("invalid value %q for --source\n  valid values: %s",
					flagSource, strings.Join(validSources, ", "))
			}

			rows, err := buildTagRows(v, flagSource, flagType, flagPrefix)
			if err != nil {
				return err
			}

			total := len(rows)
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			if flagJSON {
				b, err := json.Marshal(projectRows(rows, flagSource))
				if err != nil {
					return err
				}
				cmd.Println(string(b))
			} else {
				for _, r := range rows {
					if flagSource == "defined" {
						cmd.Println(r.Tag)
					} else {
						cmd.Printf("%d\t%s\n", r.Count, r.Tag)
					}
				}
			}
			if hint := output.TruncationHint("by count", len(rows), total,
				[]string{"--prefix", "--type", "--source", "--limit N"}); hint != "" {
				cmd.PrintErrln(hint)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "filter to one artifact type (e.g. learning)")
	cmd.Flags().StringVar(&flagPrefix, "prefix", "", "filter tags by prefix (e.g. domain/)")
	cmd.Flags().StringVar(&flagSource, "source", "used", "tag source: used | defined | all")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "maximum results to return (default 50)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON")
	return cmd
}

func projectRows(rows []tagRow, source string) any {
	switch source {
	case "used":
		out := make([]struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		}, len(rows))
		for i, r := range rows {
			out[i].Tag = r.Tag
			out[i].Count = r.Count
		}
		return out
	case "defined":
		out := make([]struct {
			Tag     string `json:"tag"`
			Defined bool   `json:"defined"`
		}, len(rows))
		for i, r := range rows {
			out[i].Tag = r.Tag
			out[i].Defined = true
		}
		return out
	default:
		return rows
	}
}

func buildTagRows(v *core.Vault, source, typeFlag, prefix string) ([]tagRow, error) {
	used := map[string]int{}
	if source == "used" || source == "all" {
		var typeFilter *core.Type
		if typeFlag != "" {
			t, err := core.ParseType(typeFlag)
			if err != nil {
				return nil, err
			}
			typeFilter = &t
		}
		types := core.AllTypes
		if typeFilter != nil {
			types = []core.Type{*typeFilter}
		}
		seenDirs := map[string]struct{}{}
		for _, t := range types {
			dir := filepath.Join(v.Root, t.Dir())
			if _, dup := seenDirs[dir]; dup {
				continue
			}
			seenDirs[dir] = struct{}{}
			if err := walkTags(dir, typeFilter, used); err != nil {
				return nil, err
			}
		}
	}

	defined := map[string]bool{}
	if source == "defined" || source == "all" {
		g, err := glossary.Load(glossary.Path(v.Root))
		if err != nil {
			return nil, err
		}
		for _, t := range g.Tags() {
			defined[t] = true
		}
	}

	keys := map[string]struct{}{}
	switch source {
	case "used":
		for k := range used {
			keys[k] = struct{}{}
		}
	case "defined":
		for k := range defined {
			keys[k] = struct{}{}
		}
	case "all":
		for k := range used {
			keys[k] = struct{}{}
		}
		for k := range defined {
			keys[k] = struct{}{}
		}
	}

	rows := make([]tagRow, 0, len(keys))
	for k := range keys {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		r := tagRow{Tag: k}
		if source != "defined" {
			r.Count = used[k] // zero for defined-only tags under source=all
		}
		if source == "all" {
			r.Defined = defined[k]
		}
		if source == "defined" {
			r.Defined = true
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if source == "defined" {
			return rows[i].Tag < rows[j].Tag
		}
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Tag < rows[j].Tag
	})
	return rows, nil
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
