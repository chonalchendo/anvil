package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <source-type> <source-id> <target-type> <target-id>",
		Short: "Append a wikilink from a source artifact to a target artifact",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return fmt.Errorf("re-loading source after link: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("indexing source: %w", err)
			}
			cmd.Println(fmt.Sprintf("linked %s.%s → %s.%s", src, srcID, tgt, tgtID))
			return nil
		},
	}
}
