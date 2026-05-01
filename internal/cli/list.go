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

type listItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Path   string `json:"path"`
}

type listFilters struct {
	Status, Project, Tag string
	TagsAllOf            []string
	Diataxis, Confidence string
}

func newListCmd() *cobra.Command {
	var (
		flagStatus, flagProject, flagTag string
		flagDiataxis, flagConfidence     string
		flagTags                         []string
		flagJSON                         bool
	)

	cmd := &cobra.Command{
		Use:   "list <type>",
		Short: "List vault artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			var tagsAllOf []string
			for _, raw := range flagTags {
				for _, tag := range strings.Split(raw, ",") {
					if tag = strings.TrimSpace(tag); tag != "" {
						tagsAllOf = append(tagsAllOf, tag)
					}
				}
			}

			return runList(cmd, v, t, listFilters{
				Status:     flagStatus,
				Project:    flagProject,
				Tag:        flagTag,
				TagsAllOf:  tagsAllOf,
				Diataxis:   flagDiataxis,
				Confidence: flagConfidence,
			}, flagJSON)
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "filter by status (exact match)")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter by project (exact match)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag (substring match, single)")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "filter by tags (all-of, exact, comma-separated)")
	cmd.Flags().StringVar(&flagDiataxis, "diataxis", "", "filter by diataxis (exact match)")
	cmd.Flags().StringVar(&flagConfidence, "confidence", "", "filter by confidence (exact match)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	return cmd
}

func runList(cmd *cobra.Command, v *core.Vault, t core.Type, f listFilters, asJSON bool) error {
	dir := filepath.Join(v.Root, t.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	var items []listItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		a, err := core.LoadArtifact(path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", e.Name(), err)
		}
		id := strings.TrimSuffix(e.Name(), ".md")

		status, _ := a.FrontMatter["status"].(string)
		project, _ := a.FrontMatter["project"].(string)
		title, _ := a.FrontMatter["title"].(string)
		diataxis, _ := a.FrontMatter["diataxis"].(string)
		confidence, _ := a.FrontMatter["confidence"].(string)

		if f.Status != "" && status != f.Status {
			continue
		}
		if f.Project != "" && project != f.Project {
			continue
		}
		if f.Diataxis != "" && diataxis != f.Diataxis {
			continue
		}
		if f.Confidence != "" && confidence != f.Confidence {
			continue
		}
		if f.Tag != "" && !hasTagSubstring(a.FrontMatter["tags"], f.Tag) {
			continue
		}
		if len(f.TagsAllOf) > 0 && !hasAllTags(a.FrontMatter["tags"], f.TagsAllOf) {
			continue
		}

		items = append(items, listItem{
			ID:     id,
			Type:   string(t),
			Title:  title,
			Status: status,
			Path:   path,
		})
	}

	// os.ReadDir returns entries sorted by name; sort explicitly to be safe.
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	if asJSON {
		if items == nil {
			items = []listItem{}
		}
		b, _ := json.Marshal(items)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	for _, item := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.ID, item.Status, item.Title)
	}
	return nil
}

// hasTagSubstring reports whether any element of tags (a []any from YAML) contains sub.
func hasTagSubstring(tags any, sub string) bool {
	list, ok := tags.([]any)
	if !ok {
		return false
	}
	for _, tag := range list {
		if s, ok := tag.(string); ok && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// hasAllTags reports whether tags contains every element of want (exact match).
func hasAllTags(tags any, want []string) bool {
	list, ok := tags.([]any)
	if !ok {
		return false
	}
	have := make(map[string]struct{}, len(list))
	for _, tag := range list {
		if s, ok := tag.(string); ok {
			have[s] = struct{}{}
		}
	}
	for _, w := range want {
		if _, ok := have[w]; !ok {
			return false
		}
	}
	return true
}
