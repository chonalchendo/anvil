package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// issueWalkability is one issue's spine-walk verdict: whether every edge from
// the issue resolves (Walkable) and, when not, which edges are broken
// (dangling link) or empty (resolves but the target body is a stub). Reuses
// hydrate's assembleHydration walk — this is the same walk aggregated over
// every issue in the vault rather than run for one.
type issueWalkability struct {
	ID          string   `json:"id"`
	Walkable    bool     `json:"walkable"`
	BrokenEdges []string `json:"broken_edges,omitempty"`
	EmptyEdges  []string `json:"empty_edges,omitempty"`
}

func newWalkabilityCmd() *cobra.Command {
	var flagJSON bool
	var flagProject string
	cmd := &cobra.Command{
		Use:     "walkability",
		Short:   "Vault-wide spine walkability report: names every issue's broken (dangling link) or empty (stub body) box edges",
		Args:    cobra.NoArgs,
		Example: "  anvil walkability\n  anvil walkability --project anvil --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			report, err := runWalkabilityReport(v, flagProject)
			if err != nil {
				return err
			}
			return emitWalkability(cmd, report, flagJSON)
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, `emit JSON envelope: {"items":[...],"total":<int>,"returned":<int>,"truncated":<bool>}`)
	cmd.Flags().StringVar(&flagProject, "project", "", "filter to one project's issues (exact match)")
	return cmd
}

// runWalkabilityReport walks every issue's methodology spine (the same walk
// assembleHydration performs for one issue) and returns a per-issue verdict.
// Unreadable artifacts encountered mid-walk abort the report — a report that
// silently drops issues on I/O error would misreport the aggregate.
func runWalkabilityReport(v *core.Vault, project string) ([]issueWalkability, error) {
	paths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return nil, fmt.Errorf("reading issues: %w", err)
	}

	var out []issueWalkability
	for _, p := range paths {
		id := listIDFor(core.TypeIssue, p)
		if project != "" {
			a, err := core.LoadArtifact(p)
			if err != nil {
				return nil, fmt.Errorf("loading issue %s: %w", id, err)
			}
			if proj, _ := a.FrontMatter["project"].(string); proj != project {
				continue
			}
		}

		h, err := assembleHydration(v, id)
		if err != nil {
			return nil, fmt.Errorf("walking issue %s: %w", id, err)
		}
		out = append(out, walkabilityOf(id, h))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// walkabilityOf reduces one issue's hydration walk to its verdict. Node 0 is
// always the issue itself (assembleHydration seeds it first) — its own body
// is not an edge target, so emptiness only applies to nodes[1:].
func walkabilityOf(id string, h *hydration) issueWalkability {
	w := issueWalkability{ID: id}
	for _, e := range h.broken {
		w.BrokenEdges = append(w.BrokenEdges, fmt.Sprintf("%s → [[%s]] (target not found)", e.Source, e.Target))
	}
	for _, n := range h.nodes[1:] {
		if strings.TrimSpace(n.Body) == "" {
			w.EmptyEdges = append(w.EmptyEdges, fmt.Sprintf("%s %s (stub body)", n.Type, n.ID))
		}
	}
	w.Walkable = len(w.BrokenEdges) == 0 && len(w.EmptyEdges) == 0
	return w
}

// emitWalkability prints the report: --json emits the standard bounded-list
// envelope (unbounded here, so returned always equals total); the default
// human view is one line per issue, naming each broken/empty edge indented
// beneath its issue.
func emitWalkability(cmd *cobra.Command, report []issueWalkability, asJSON bool) error {
	if asJSON {
		items := report
		if items == nil {
			items = []issueWalkability{}
		}
		env := struct {
			Items     []issueWalkability `json:"items"`
			Total     int                `json:"total"`
			Returned  int                `json:"returned"`
			Truncated bool               `json:"truncated"`
		}{items, len(items), len(items), false}
		b, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("marshalling json: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}

	w := cmd.OutOrStdout()
	walkable := 0
	for _, r := range report {
		if r.Walkable {
			walkable++
			fmt.Fprintf(w, "%s walkable\n", r.ID)
			continue
		}
		fmt.Fprintf(w, "%s NOT walkable\n", r.ID)
		for _, e := range r.BrokenEdges {
			fmt.Fprintf(w, "  broken: %s\n", e)
		}
		for _, e := range r.EmptyEdges {
			fmt.Fprintf(w, "  empty:  %s\n", e)
		}
	}
	cmd.PrintErrf("%d/%d issue(s) walkable\n", walkable, len(report))
	return nil
}
