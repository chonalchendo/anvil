package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
)

func newTagsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tags",
		Short:        "Inspect and curate vault tags",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newTagsListCmd(), newTagsAddCmd(), newTagsDefineCmd())
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

			if flagLimit < 1 || flagLimit > 50 {
				return fmt.Errorf("invalid value %d for --limit\n  valid values: 1..50", flagLimit)
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

			rows, glossaryLoaded, err := buildTagRows(v, flagSource, flagType, flagPrefix)
			if err != nil {
				return err
			}

			total := len(rows)
			if len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			// Data output goes to stdout via OutOrStdout(); cmd.Println /
			// cmd.Printf default to stderr unless SetOut was called, which
			// breaks `anvil tags list --json | jq ...` for agent pipelines.
			out := cmd.OutOrStdout()
			if flagJSON {
				b, err := json.Marshal(projectRows(rows, flagSource, glossaryLoaded))
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else {
				for _, r := range rows {
					switch {
					case flagSource == "defined":
						fmt.Fprintln(out, r.Tag)
					case flagSource == "used" && glossaryLoaded && !r.Defined:
						fmt.Fprintf(out, "%d\t%s (undefined)\n", r.Count, r.Tag)
					default:
						fmt.Fprintf(out, "%d\t%s\n", r.Count, r.Tag)
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

// projectRows shapes rows for JSON output. For --source used with a non-empty
// glossary, emit {tag, count, defined} so callers can detect drift without
// running validate. For a fresh vault (rows carry zero-value Defined), keep
// the compact {tag, count} shape to avoid misleading false:false output.
func projectRows(rows []tagRow, source string, glossaryLoaded bool) any {
	switch source {
	case "used":
		if glossaryLoaded {
			out := make([]struct {
				Tag     string `json:"tag"`
				Count   int    `json:"count"`
				Defined bool   `json:"defined"`
			}, len(rows))
			for i, r := range rows {
				out[i].Tag = r.Tag
				out[i].Count = r.Count
				out[i].Defined = r.Defined
			}
			return out
		}
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

// buildTagRows returns the tag rows and a bool indicating whether the glossary
// was loaded with at least one entry (used by callers to decide whether to show
// the "defined" field or the "(undefined)" suffix).
func buildTagRows(v *core.Vault, source, typeFlag, prefix string) ([]tagRow, bool, error) {
	used := map[string]int{}
	if source == "used" || source == "all" {
		var typeFilter *core.Type
		if typeFlag != "" {
			t, err := core.ParseType(typeFlag)
			if err != nil {
				return nil, false, err
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
				return nil, false, err
			}
		}
	}

	defined := map[string]bool{}
	glossaryLoaded := false
	if source == "defined" || source == "all" || source == "used" {
		g, err := glossary.Load(glossary.Path(v.Root))
		if err != nil {
			return nil, false, err
		}
		if tags := g.Tags(); len(tags) > 0 {
			glossaryLoaded = true
			for _, t := range tags {
				defined[t] = true
			}
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
		if source == "all" || (source == "used" && glossaryLoaded) {
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
	return rows, glossaryLoaded, nil
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

func newTagsAddCmd() *cobra.Command {
	var (
		flagDesc   string
		flagUpdate bool
	)
	cmd := &cobra.Command{
		Use:   "add <facet>/<name> --desc \"...\"",
		Short: "Add a tag to the vault glossary (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]
			if flagDesc == "" {
				return fmt.Errorf("--desc is required")
			}
			facet, _, ok := glossary.SplitTag(tag)
			if !ok || !slices.Contains(glossary.Facets, facet) {
				return fmt.Errorf("invalid value %q for <facet>/<name>\n  valid values: %s\n  corrected:    anvil tags add %s/<name> --desc %q",
					tag, strings.Join(glossary.Facets, ", "), glossary.Facets[0], flagDesc)
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			path := glossary.Path(v.Root)
			g, err := glossary.Load(path)
			if err != nil {
				return err
			}
			existing, hadIt := g.FindTagDesc(tag)
			if hadIt && existing == flagDesc {
				fmt.Fprintln(cmd.OutOrStdout(), path)
				return nil
			}
			if hadIt && !flagUpdate {
				return fmt.Errorf("tag %q already defined with a different description\n  existing: %s\n  new:      %s\n  corrected: anvil tags add %s --desc %q --update",
					tag, existing, flagDesc, tag, flagDesc)
			}
			if hadIt && flagUpdate {
				// hadIt was just verified above and there are no concurrent writers.
				_ = g.UpdateTagDesc(tag, flagDesc)
			} else {
				if err := g.AddTag(tag, flagDesc); err != nil {
					return err
				}
			}
			if err := g.Save(path); err != nil {
				return fmt.Errorf("saving glossary: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDesc, "desc", "", "one-line description (required)")
	cmd.Flags().BoolVar(&flagUpdate, "update", false, "rewrite existing tag's description")
	return cmd
}

func newTagsDefineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "define <term>",
		Short: "Print the definition for <term> from _meta/glossary.md",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			g, err := glossary.Load(glossary.Path(v.Root))
			if err != nil {
				return err
			}
			def, ok := g.Definition(args[0])
			if !ok {
				return fmt.Errorf("term %q not in glossary", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), def)
			return nil
		},
	}
}
