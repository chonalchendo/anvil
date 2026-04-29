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

func newListCmd() *cobra.Command {
	var flagStatus, flagProject, flagTag string
	var flagJSON bool

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

				if flagStatus != "" && status != flagStatus {
					continue
				}
				if flagProject != "" && project != flagProject {
					continue
				}
				if flagTag != "" && !hasTagSubstring(a.FrontMatter["tags"], flagTag) {
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

			// os.ReadDir returns entries sorted by name, so items are already
			// sorted by id (filename minus .md). Sort explicitly to be safe.
			sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

			if flagJSON {
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
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "filter by status (exact match)")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter by project (exact match)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag (substring match)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	return cmd
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
