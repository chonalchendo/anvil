package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate vault frontmatter against schemas",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var root string
			if len(args) == 1 {
				root = args[0]
			} else {
				v, err := core.ResolveVault()
				if err != nil {
					return err
				}
				root = v.Root
			}

			var failures []string
			for _, t := range core.AllTypes {
				dir := filepath.Join(root, t.Dir())
				entries, err := os.ReadDir(dir)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return fmt.Errorf("read %s: %w", dir, err)
				}
				for _, e := range entries {
					if filepath.Ext(e.Name()) != ".md" {
						continue
					}
					path := filepath.Join(dir, e.Name())
					a, err := core.LoadArtifact(path)
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s: %s", path, err))
						continue
					}
					if err := schema.Validate(string(t), a.FrontMatter); err != nil {
						failures = append(failures, fmt.Sprintf("%s: %s", path, err))
					}
				}
			}

			for _, f := range failures {
				cmd.PrintErrln(f)
			}
			if len(failures) > 0 {
				return ErrSchemaInvalid
			}
			return nil
		},
	}
}
