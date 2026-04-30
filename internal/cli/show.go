package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newShowCmd() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "show <type> <id>",
		Short: "Display a vault artifact",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			return runShow(cmd, v, t, args[1], flagJSON)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	return cmd
}

// runShow displays the artifact identified by id within type t's directory.
func runShow(cmd *cobra.Command, v *core.Vault, t core.Type, id string, asJSON bool) error {
	path := filepath.Join(v.Root, t.Dir(), id+".md")

	if asJSON {
		a, err := core.LoadArtifact(path)
		if err != nil {
			if os.IsNotExist(err) {
				return ErrArtifactNotFound
			}
			return fmt.Errorf("loading artifact: %w", err)
		}
		out := make(map[string]any, len(a.FrontMatter)+2)
		for k, val := range a.FrontMatter {
			out[k] = val
		}
		out["body"] = a.Body
		out["path"] = a.Path
		b, _ := json.Marshal(out)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}

	// Text mode: raw cat of the file preserves user formatting.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrArtifactNotFound
		}
		return fmt.Errorf("reading artifact: %w", err)
	}
	fmt.Fprint(cmd.OutOrStdout(), string(data))
	return nil
}
