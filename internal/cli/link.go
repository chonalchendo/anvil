package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newLinkCmd() *cobra.Command {
	var fromID, toID, externalURI, relation string
	var unresolved, asJSON bool
	cmd := &cobra.Command{
		Use:   "link [<source-type> <source-id> <target-type> <target-id> | <source-type> <source-id> --external <uri>]",
		Short: "Append a wikilink, an external URI (--external), or query the link graph (--from/--to/--unresolved)",
		Example: "  anvil link issue demo.foo learning demo.gotcha\n" +
			"  anvil link issue demo.foo issue demo.bar --relation depends_on\n" +
			"  anvil link issue demo.foo --external https://github.com/x/y/pull/13\n" +
			"  anvil link --from demo.foo --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if trimmed := strings.TrimSpace(externalURI); trimmed != externalURI {
				if trimmed == "" {
					return fmt.Errorf("--external requires a non-blank value")
				}
				externalURI = trimmed
			}

			relationSet := cmd.Flags().Changed("relation")
			readMode := fromID != "" || toID != "" || unresolved
			if readMode {
				if len(args) > 0 || externalURI != "" || relationSet {
					return fmt.Errorf("--from/--to/--unresolved cannot be combined with positional write args, --external, or --relation")
				}
				return runLinkQuery(cmd, fromID, toID, unresolved, asJSON)
			}

			if relationSet && externalURI != "" {
				return fmt.Errorf("--relation cannot be combined with --external")
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
				srcID, srcPath := core.ResolveArtifact(v, src, args[1])
				if err := core.AppendExternalLink(v, src, srcID, externalURI); err != nil {
					return err
				}
				a, err := core.LoadArtifact(srcPath)
				if err != nil {
					return fmt.Errorf("re-loading source: %w", err)
				}
				if err := indexAfterSave(v, a); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "linked %s → %s\n", core.WikilinkTarget(src, srcID), externalURI)
				return nil
			}

			if len(args) != 4 {
				return fmt.Errorf("write form requires 4 args: source-type source-id target-type target-id")
			}
			switch relation {
			case "related", "depends_on", "blocks":
			default:
				return fmt.Errorf("--relation must be related, depends_on, or blocks (got %q)", relation)
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
			srcID, srcPath := core.ResolveArtifact(v, src, args[1])
			tgtID := args[3]
			if tgt == core.TypeIssue {
				tgtID = core.ResolveIssueArg(v, tgtID)
			}
			tgtID, err = resolveLinkTarget(v, tgt, tgtID)
			if err != nil {
				return err
			}
			if err := core.AppendLink(v, src, srcID, tgt, tgtID, relation); err != nil {
				return err
			}
			a, err := core.LoadArtifact(srcPath)
			if err != nil {
				return fmt.Errorf("re-loading source: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "linked %s → %s.%s\n", core.WikilinkTarget(src, srcID), tgt, tgtID)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromID, "from", "", "list outgoing edges from this artifact id")
	cmd.Flags().StringVar(&toID, "to", "", "list incoming edges to this artifact id")
	cmd.Flags().StringVar(&externalURI, "external", "", "append a free-form URI (commit sha, PR url, doc link) to source.external_links")
	cmd.Flags().StringVar(&relation, "relation", "related", "edge slot for the 4-arg write form: related (default), depends_on, or blocks")
	cmd.Flags().BoolVar(&unresolved, "unresolved", false, "list edges whose target is not in the vault")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	return cmd
}

// resolveLinkTarget returns the target id `AppendLink` should embed in its
// `[[<type>.<id>]]` wikilink, and refuses a target with no artifact on disk so
// a dead edge cannot be written in the first place. `anvil list` prints
// type-prefixed ids for the prefix-keeping types and bare ids for the rest,
// while body wikilinks always carry the prefix, so both forms are tried and
// whichever resolves wins.
func resolveLinkTarget(v *core.Vault, tgt core.Type, id string) (string, error) {
	// An id carrying <, >, or whitespace is an unsubstituted documentation
	// placeholder (e.g. `anvil link issue <id> system-design <project>` copied
	// verbatim). No artifact id can contain these, so refuse before consulting
	// the resolver — the link indexer would still write the edge, leaving a
	// dead one `show --validate` cannot see.
	if i := strings.IndexAny(id, "<> \t\n"); i >= 0 {
		return "", fmt.Errorf("target id %q contains %q, which no artifact id may contain; substitute the real id `anvil list %s` prints",
			id, id[i:i+1], tgt)
	}
	// Probe every rung of the prefix-strip chain, most-stripped first:
	// probing a still-prefixed form first can resolve a doubled target onto
	// the real file and silently embed the doubled wikilink (anvil.0234).
	// Keeping every rung, not just the fully-stripped one, is what lets a
	// legitimate id whose leading segment equals the type name resolve.
	prefix := string(tgt) + "."
	candidates := []string{id}
	for c, ok := strings.CutPrefix(id, prefix); ok && c != ""; c, ok = strings.CutPrefix(c, prefix) {
		candidates = append([]string{c}, candidates...)
	}
	tried := make([]string, 0, len(candidates))
	for _, c := range candidates {
		target := fmt.Sprintf("%s.%s", tgt, c)
		// Same on-disk lookup `--validate` uses, so the write path and the
		// validator can never disagree about what a live edge is.
		if core.WikilinkTargetExists(v, target) {
			return c, nil
		}
		tried = append(tried, "[["+target+"]]")
	}
	return "", fmt.Errorf("no %s artifact for %q (tried %s); pass the id `anvil list %s` prints",
		tgt, id, strings.Join(tried, " and "), tgt)
}

type linkRowOut struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
	Anchor   string `json:"anchor,omitempty"`
	Path     string `json:"path"`
}

func runLinkQuery(cmd *cobra.Command, fromID, toID string, unresolved, asJSON bool) error {
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
	if count > 1 {
		return fmt.Errorf("--from, --to, --unresolved are mutually exclusive")
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
