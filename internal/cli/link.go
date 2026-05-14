package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newLinkCmd() *cobra.Command {
	var fromID, toID, externalURI string
	var unresolved, drift, asJSON bool
	cmd := &cobra.Command{
		Use:   "link [<source-type> <source-id> <target-type> <target-id> | <source-type> <source-id> --external <uri>]",
		Short: "Append a wikilink, an external URI (--external), or query the link graph (--from/--to/--unresolved/--drift)",
		Example: "  anvil link issue demo.foo learning demo.gotcha\n" +
			"  anvil link issue demo.foo --external https://github.com/x/y/pull/13\n" +
			"  anvil link --from demo.foo --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if trimmed := strings.TrimSpace(externalURI); trimmed != externalURI {
				if trimmed == "" {
					return fmt.Errorf("--external requires a non-blank value")
				}
				externalURI = trimmed
			}

			readMode := fromID != "" || toID != "" || unresolved || drift
			if readMode {
				if len(args) > 0 || externalURI != "" {
					return fmt.Errorf("--from/--to/--unresolved/--drift cannot be combined with positional write args or --external")
				}
				return runLinkQuery(cmd, fromID, toID, unresolved, drift, asJSON)
			}

			if externalURI != "" {
				if len(args) != 2 {
					return fmt.Errorf("--external form requires 2 args: source-type source-id (got %d)", len(args))
				}
				src, err := core.ParseType(args[0])
				if err != nil {
					return fmt.Errorf("source type: %w", err)
				}
				v, err := core.ResolveVault()
				if err != nil {
					return fmt.Errorf("resolving vault: %w", err)
				}
				srcID := args[1]
				if err := core.AppendExternalLink(v, src, srcID, externalURI); err != nil {
					return err
				}
				srcPath := filepath.Join(v.Root, src.Dir(), srcID+".md")
				a, err := core.LoadArtifact(srcPath)
				if err != nil {
					return fmt.Errorf("re-loading source: %w", err)
				}
				if err := indexAfterSave(v, a); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "linked %s.%s → %s\n", src, srcID, externalURI)
				return nil
			}

			if len(args) != 4 {
				return fmt.Errorf("write form requires 4 args: source-type source-id target-type target-id")
			}
			src, err := core.ParseType(args[0])
			if err != nil {
				return fmt.Errorf("source type: %w", err)
			}
			tgt, err := core.ParseType(args[2])
			if err != nil {
				return fmt.Errorf("target type: %w", err)
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			srcID, tgtID := args[1], args[3]
			if err := core.AppendLink(v, src, srcID, tgt, tgtID); err != nil {
				return err
			}
			srcPath := filepath.Join(v.Root, src.Dir(), srcID+".md")
			a, err := core.LoadArtifact(srcPath)
			if err != nil {
				return fmt.Errorf("re-loading source: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "linked %s.%s → %s.%s\n", src, srcID, tgt, tgtID)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromID, "from", "", "list outgoing edges from this artifact id")
	cmd.Flags().StringVar(&toID, "to", "", "list incoming edges to this artifact id")
	cmd.Flags().StringVar(&externalURI, "external", "", "append a free-form URI (commit sha, PR url, doc link) to source.external_links")
	cmd.Flags().BoolVar(&unresolved, "unresolved", false, "list edges whose target is not in the vault")
	cmd.Flags().BoolVar(&drift, "drift", false, "list plan→issue pairs whose slugs disagree")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	return cmd
}

// runLinkDrift emits plan→issue pairs whose slugs disagree. Output format
// mirrors the other link-query subcommands so downstream JSON consumers see
// a consistent shape (plus the slug pair so the caller doesn't have to
// re-parse ids).
func runLinkDrift(cmd *cobra.Command, db *index.DB, asJSON bool) error {
	rows, err := db.LinksSlugDrift()
	if err != nil {
		return err
	}
	type driftRow struct {
		Source     string `json:"source"`
		Target     string `json:"target"`
		Relation   string `json:"relation"`
		SourceSlug string `json:"source_slug"`
		TargetSlug string `json:"target_slug"`
		Path       string `json:"path,omitempty"`
	}
	out := make([]driftRow, 0, len(rows))
	for _, r := range rows {
		path := ""
		if a, err := db.GetArtifact(r.Source); err == nil {
			path = a.Path
		}
		out = append(out, driftRow{
			Source:     r.Source,
			Target:     r.Target,
			Relation:   r.Relation,
			SourceSlug: slugPart(r.Source),
			TargetSlug: slugPart(r.Target),
			Path:       path,
		})
	}
	if asJSON {
		b, _ := json.Marshal(out)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	for _, r := range out {
		fmt.Fprintf(cmd.OutOrStdout(), "drift %s (%s) -> %s (%s)\n",
			r.Source, r.SourceSlug, r.Target, r.TargetSlug)
	}
	return nil
}

// slugPart returns the portion of id after the first dot, or id itself when
// it has no project prefix.
func slugPart(id string) string {
	if i := strings.IndexByte(id, '.'); i >= 0 {
		return id[i+1:]
	}
	return id
}

type linkRowOut struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
	Anchor   string `json:"anchor,omitempty"`
	Path     string `json:"path"`
}

func runLinkQuery(cmd *cobra.Command, fromID, toID string, unresolved, drift, asJSON bool) error {
	count := 0
	if fromID != "" {
		count++
	}
	if toID != "" {
		count++
	}
	if unresolved {
		count++
	}
	if drift {
		count++
	}
	if count > 1 {
		return fmt.Errorf("--from, --to, --unresolved, --drift are mutually exclusive")
	}
	v, err := core.ResolveVault()
	if err != nil {
		return fmt.Errorf("resolving vault: %w", err)
	}
	db, err := indexForRead(v)
	if err != nil {
		return err
	}
	defer db.Close()

	if drift {
		return runLinkDrift(cmd, db, asJSON)
	}

	var rows []index.LinkRow
	switch {
	case fromID != "":
		rows, err = db.LinksFrom(fromID)
	case toID != "":
		rows, err = db.LinksTo(toID)
	case unresolved:
		rows, err = db.LinksUnresolved()
	}
	if err != nil {
		return err
	}

	out := make([]linkRowOut, 0, len(rows))
	for _, r := range rows {
		path := ""
		if a, err := db.GetArtifact(r.Source); err == nil {
			path = a.Path
		}
		out = append(out, linkRowOut{
			Source: r.Source, Target: r.Target, Relation: r.Relation,
			Anchor: r.Anchor, Path: path,
		})
	}
	if asJSON {
		b, _ := json.Marshal(out)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	for _, r := range out {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s -> %s\n", r.Relation, r.Source, r.Target)
	}
	return nil
}

