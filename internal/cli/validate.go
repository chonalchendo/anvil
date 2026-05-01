package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
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

			g, err := glossary.Load(glossary.Path(root))
			if err != nil {
				return fmt.Errorf("loading glossary: %w", err)
			}
			var known map[string]struct{}
			if tags := g.Tags(); len(tags) > 0 {
				known = make(map[string]struct{}, len(tags))
				for _, tag := range tags {
					known[tag] = struct{}{}
				}
			}
			if known != nil {
				known["type/learning"] = struct{}{}
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
						continue
					}
					if t == core.TypeLearning {
						for _, vErr := range core.ValidateLearning(a, known) {
							failures = append(failures, fmt.Sprintf("%s: %s", path, vErr))
						}
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
