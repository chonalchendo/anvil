package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
)

const defaultListLimit = 10

type listItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Created     string   `json:"created,omitempty"`
	Project     string   `json:"project,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Path        string   `json:"path"`
}

type listFilters struct {
	Status, Project, Tag string
	TagsAllOf            []string
	Diataxis, Confidence string
	Since, Until         string
}

func newListCmd() *cobra.Command {
	var (
		flagStatus, flagProject, flagTag string
		flagDiataxis, flagConfidence     string
		flagSince, flagUntil             string
		flagTags                         []string
		flagJSON                         bool
		flagLimit                        int
	)

	cmd := &cobra.Command{
		Use:     "list <type>",
		Short:   "List vault artifacts (default: 10 most recent)",
		Args:    cobra.ExactArgs(1),
		Example: "  anvil list issue --status open\n  anvil list plan --since 2026-05-01 --limit 25\n  anvil list decision --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			tagsAllOf := splitTags(flagTags)
			return runList(cmd, v, t, listFilters{
				Status: flagStatus, Project: flagProject, Tag: flagTag,
				TagsAllOf: tagsAllOf, Diataxis: flagDiataxis,
				Confidence: flagConfidence, Since: flagSince, Until: flagUntil,
			}, flagJSON, flagLimit)
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "filter by status (exact match)")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter by project (exact match)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag (substring match, single)")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "filter by tags (all-of, exact, comma-separated)")
	cmd.Flags().StringVar(&flagDiataxis, "diataxis", "", "filter by diataxis (exact match)")
	cmd.Flags().StringVar(&flagConfidence, "confidence", "", "filter by confidence (exact match)")
	cmd.Flags().StringVar(&flagSince, "since", "", "include only artifacts created on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&flagUntil, "until", "", "include only artifacts created on or before YYYY-MM-DD")
	cmd.Flags().IntVar(&flagLimit, "limit", defaultListLimit, "maximum results to return (default 10)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	return cmd
}

func splitTags(raw []string) []string {
	var out []string
	for _, r := range raw {
		for _, t := range strings.Split(r, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

func runList(cmd *cobra.Command, v *core.Vault, t core.Type, f listFilters, asJSON bool, limit int) error {
	dir := filepath.Join(v.Root, t.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return emitList(cmd, nil, 0, asJSON)
		}
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
		description, _ := a.FrontMatter["description"].(string)
		created, _ := a.FrontMatter["created"].(string)
		diataxis, _ := a.FrontMatter["diataxis"].(string)
		confidence, _ := a.FrontMatter["confidence"].(string)

		if !matchesFilters(f, status, project, diataxis, confidence, created, a.FrontMatter["tags"]) {
			continue
		}

		items = append(items, listItem{
			ID: id, Type: string(t), Title: title, Description: description,
			Status: status, Created: created, Project: project,
			Tags: stringTags(a.FrontMatter["tags"]), Path: path,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Created != items[j].Created {
			return items[i].Created > items[j].Created
		}
		return items[i].ID < items[j].ID
	})

	total := len(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return emitList(cmd, items, total, asJSON)
}

func matchesFilters(f listFilters, status, project, diataxis, confidence, created string, tagsRaw any) bool {
	if f.Status != "" && status != f.Status {
		return false
	}
	if f.Project != "" && project != f.Project {
		return false
	}
	if f.Diataxis != "" && diataxis != f.Diataxis {
		return false
	}
	if f.Confidence != "" && confidence != f.Confidence {
		return false
	}
	if f.Tag != "" && !hasTagSubstring(tagsRaw, f.Tag) {
		return false
	}
	if len(f.TagsAllOf) > 0 && !hasAllTags(tagsRaw, f.TagsAllOf) {
		return false
	}
	if f.Since != "" && created < f.Since {
		return false
	}
	if f.Until != "" && created > f.Until {
		return false
	}
	return true
}

func emitList(cmd *cobra.Command, items []listItem, total int, asJSON bool) error {
	returned := len(items)
	if asJSON {
		return output.WriteListJSON(cmd.OutOrStdout(), items, total, returned)
	}
	w := cmd.OutOrStdout()
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", item.ID, item.Status, firstNonEmpty(item.Description, item.Title))
	}
	if hint := output.TruncationHint("most recent", returned, total,
		[]string{"--since/--until", "--status", "--type", "--tag", "--project"}); hint != "" {
		cmd.PrintErrln(hint)
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func stringTags(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, t := range list {
		if s, ok := t.(string); ok {
			out = append(out, s)
		}
	}
	return out
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
