package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// relatedOut is the JSON shape of one `anvil index` result. shared_tags and
// links default to [] (never null) so agent jq pipelines can index them freely.
type relatedOut struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Status     string   `json:"status"`
	Project    string   `json:"project"`
	Path       string   `json:"path"`
	Score      int      `json:"score"`
	SharedTags []string `json:"shared_tags"`
	Links      []string `json:"links"`
}

func newIndexCmd() *cobra.Command {
	var (
		flagTags    string
		flagType    string
		flagProject string
		flagStatus  string
		flagLimit   int
		flagJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "index [<id>] [--tags <facet/value,...>]",
		Short: "Surface artifacts related to a seed by shared facets + link adjacency",
		Long: `Seed is either a positional artifact id or a --tags set (exactly one).
Results rank artifacts by shared tags — plus a bonus for a direct link to an id
seed — each carrying the matched tags/links as evidence. Read-only; no LLM.`,
		Example: `  anvil index anvil.0042.some-issue --json
  anvil index --tags domain/cli,activity/issue --json`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			hasID := len(args) == 1
			hasTags := flagTags != ""
			if hasID == hasTags {
				return fmt.Errorf("provide exactly one seed: an <id> argument or --tags <facet/value,...>")
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := indexForRead(v)
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			qf := index.QueryFilters{Status: flagStatus, Project: flagProject}
			var rows []index.RelatedRow
			if hasID {
				if _, err := db.GetArtifact(args[0]); err != nil {
					if errors.Is(err, index.ErrArtifactNotInIndex) {
						return fmt.Errorf("unknown artifact id %q — check `anvil list` or reindex", args[0])
					}
					return err
				}
				rows, err = db.RelatedByID(args[0], qf)
			} else {
				var tags []string
				for _, p := range strings.Split(flagTags, ",") {
					if t := strings.TrimSpace(p); t != "" {
						tags = append(tags, t)
					}
				}
				rows, err = db.RelatedByTags(tags, qf)
			}
			if err != nil {
				return err
			}

			if flagType != "" {
				filtered := rows[:0]
				for _, r := range rows {
					if r.Type == flagType {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
			}

			total := len(rows)
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			// Data to stdout via OutOrStdout(); cobra's cmd.Print* default to
			// stderr, which breaks `anvil index ... --json | jq`.
			out := cmd.OutOrStdout()
			if flagJSON {
				payload := make([]relatedOut, len(rows))
				for i, r := range rows {
					payload[i] = relatedOut{
						ID: r.ID, Type: r.Type, Status: r.Status, Project: r.Project,
						Path: r.Path, Score: r.Score,
						SharedTags: orEmpty(r.SharedTags), Links: orEmpty(r.Links),
					}
				}
				b, err := json.Marshal(payload)
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else {
				for _, r := range rows {
					why := append([]string{}, r.SharedTags...)
					for _, l := range r.Links {
						why = append(why, "link:"+l)
					}
					fmt.Fprintf(out, "%d\t%s\t%s\t%s\n", r.Score, r.ID, r.Type, strings.Join(why, " "))
				}
			}

			if hint := output.TruncationHint("by relatedness", len(rows), total,
				[]string{"--type", "--project", "--status", "--limit N"}); hint != "" {
				cmd.PrintErrln(hint)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTags, "tags", "", "seed: comma-separated facet/value tags (alternative to <id>)")
	cmd.Flags().StringVar(&flagType, "type", "", "filter results to one artifact type (e.g. learning)")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter results to one project")
	cmd.Flags().StringVar(&flagStatus, "status", "", "filter results to one status")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "maximum results to return; 0 = all")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON")
	return cmd
}

// orEmpty returns a non-nil slice so JSON marshals [] rather than null.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
