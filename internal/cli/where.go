package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newWhereCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "where",
		Short: "Print vault and project paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVaultPath()
			if err != nil {
				return err
			}
			p, perr := core.ResolveProject()
			if asJSON {
				out := map[string]string{"vault": v.Root}
				if perr == nil {
					out["project"] = p.Slug
					out["project_root"] = p.Root
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "vault:", v.Root+vaultMissingNote(v))
			if perr == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "project:", p.Slug)
				fmt.Fprintln(cmd.OutOrStdout(), "project_root:", p.Root)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "project: <none>")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// vaultMissingNote flags a root Scaffold never populated, so `where` does not
// present a typo'd $ANVIL_VAULT as if it were a live vault.
func vaultMissingNote(v *core.Vault) string {
	if !core.IsVault(v.Root) {
		return " (missing — run anvil init or pass --vault)"
	}
	return ""
}
