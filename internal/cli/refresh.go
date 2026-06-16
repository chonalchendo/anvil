package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "refresh",
		Short:        "Drive freshness transitions on vault artifacts",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newRefreshLearningsCmd())
	return cmd
}

func newRefreshLearningsCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "learnings",
		Short: "Mark verified/draft learnings stale when a related-link target is gone",
		Long: `Drive the deterministic freshness signal: a verified learning whose
related wikilink targets a moved or deleted artifact is transitioned to
stale (verified→stale is the only legal edge into stale).

Only learnings eligible for →stale are examined; the judgement calls
(keep / update / consolidate / replace / delete) belong to the
refreshing-learnings skill. Designed to run unattended in vault-hygiene.`,
		Example: `  anvil refresh learnings
  anvil refresh learnings --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := indexForRead(v)
			if err != nil {
				return err
			}
			candidates, checked, err := staleLearnings(db)
			db.Close() //nolint:errcheck,gosec // read done; reopened below for the post-write reindex
			if err != nil {
				return err
			}

			result := refreshResult{Checked: checked, Transitioned: []transitioned{}}
			date := time.Now().UTC().Format("2006-01-02")
			for _, c := range candidates {
				a, err := core.LoadArtifact(c.Path)
				if err != nil {
					return fmt.Errorf("loading %s: %w", c.ID, err)
				}
				a.FrontMatter["status"] = "stale"
				a.FrontMatter["updated"] = date
				if err := a.Save(); err != nil {
					return fmt.Errorf("saving %s: %w", c.ID, err)
				}
				result.Transitioned = append(result.Transitioned, transitioned{ID: c.ID, To: "stale", Missing: c.Missing})
			}

			// One reindex absorbs every status change so the index stays
			// consistent for the next read.
			if len(result.Transitioned) > 0 {
				rdb, err := index.Open(index.DBPath(v.Root))
				if err != nil {
					return fmt.Errorf("opening index: %w", err)
				}
				defer rdb.Close() //nolint:errcheck // close in defer; error not actionable
				if _, err := rdb.Reindex(v.Root); err != nil {
					return fmt.Errorf("reindex after refresh: %w", err)
				}
			}

			out := cmd.OutOrStdout()
			if flagJSON {
				b, err := json.Marshal(result)
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintf(out, "checked %d learning(s), transitioned %d to stale\n", result.Checked, len(result.Transitioned))
			for _, t := range result.Transitioned {
				fmt.Fprintf(out, "  %s → stale (missing related: %v)\n", t.ID, t.Missing)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON")
	return cmd
}

type refreshResult struct {
	Checked      int            `json:"checked"`
	Transitioned []transitioned `json:"transitioned"`
}

type transitioned struct {
	ID      string   `json:"id"`
	To      string   `json:"to"`
	Missing []string `json:"missing"`
}

type staleCandidate struct {
	ID      string
	Path    string
	Missing []string
}

// staleLearnings returns learnings eligible for →stale (currently verified)
// that have a related-link target no longer present in the index, and the
// count examined. The target is the deterministic drift signal: a wikilink in
// `related:` pointing at a moved or deleted artifact.
func staleLearnings(db *index.DB) ([]staleCandidate, int, error) {
	learnings, err := db.ListByType(string(core.TypeLearning))
	if err != nil {
		return nil, 0, err
	}
	var out []staleCandidate
	checked := 0
	for _, l := range learnings {
		// Only learnings for which →stale is a legal edge are eligible; the
		// state machine has verified→stale but no draft→stale (a draft hasn't
		// been confirmed, so it can't go stale — the skill promotes it first).
		if _, err := core.LookupTransition(core.TypeLearning, l.Status, "stale"); err != nil {
			continue
		}
		checked++
		links, err := db.LinksFrom(l.ID)
		if err != nil {
			return nil, 0, err
		}
		var missing []string
		for _, lk := range links {
			if lk.Relation != "related" {
				continue
			}
			if _, err := db.GetArtifact(lk.Target); err != nil {
				if errors.Is(err, index.ErrArtifactNotInIndex) {
					missing = append(missing, lk.Target)
					continue
				}
				return nil, 0, err
			}
		}
		if len(missing) > 0 {
			out = append(out, staleCandidate{ID: l.ID, Path: l.Path, Missing: missing})
		}
	}
	return out, checked, nil
}
