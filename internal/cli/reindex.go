package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newReindexCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild .anvil/vault.db from the vault on disk",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return err
			}
			defer db.Close()
			stats, err := db.Reindex(v.Root)
			if err != nil {
				return err
			}
			if asJSON {
				b, _ := json.Marshal(map[string]any{
					"artifacts":   stats.Artifacts,
					"links":       stats.Links,
					"duration_ms": stats.DurationMS,
				})
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			cmd.Println(fmt.Sprintf("reindexed: %d artifacts, %d links (%dms)", stats.Artifacts, stats.Links, stats.DurationMS))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	return cmd
}
