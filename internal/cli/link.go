package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newLinkCmd() *cobra.Command {
	var fromID, toID string
	var unresolved, asJSON bool
	cmd := &cobra.Command{
		Use:   "link [<source-type> <source-id> <target-type> <target-id>]",
		Short: "Append a wikilink, or query the link graph (--from/--to/--unresolved)",
		RunE: func(cmd *cobra.Command, args []string) error {
			readMode := fromID != "" || toID != "" || unresolved
			if readMode {
				if len(args) > 0 {
					return fmt.Errorf("--from/--to/--unresolved cannot be combined with positional write args")
				}
				return runLinkQuery(cmd, fromID, toID, unresolved, asJSON)
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
	cmd.Flags().BoolVar(&unresolved, "unresolved", false, "list edges whose target is not in the vault")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	return cmd
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
	defer db.Close()

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

