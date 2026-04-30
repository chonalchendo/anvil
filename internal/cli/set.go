package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <type> <id> <field> <value>",
		Short: "Set a scalar frontmatter field on a vault artifact",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			path := filepath.Join(v.Root, t.Dir(), args[1]+".md")
			a, err := core.LoadArtifact(path)
			if err != nil {
				if os.IsNotExist(err) {
					return ErrArtifactNotFound
				}
				return fmt.Errorf("loading artifact: %w", err)
			}

			field, value := args[2], args[3]

			// Reject non-scalar existing values: lists, maps, and dates are
			// not settable via the CLI in v0.1 — use an editor.
			prev, hadPrev := a.FrontMatter[field]
			if hadPrev {
				switch prev.(type) {
				case []any, []string:
					return fmt.Errorf("field %s is a list; use editor", field)
				case map[string]any:
					return fmt.Errorf("field %s is an object; use editor", field)
				case time.Time:
					return fmt.Errorf("field %s is a date; use editor", field)
				}
			}

			a.FrontMatter[field] = value

			if err := schema.Validate(string(t), a.FrontMatter); err != nil {
				return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
			}

			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}

			if t == core.TypePlan && field == "status" && value == "locked" {
				p, lerr := core.LoadPlan(a.Path)
				if lerr != nil {
					return fmt.Errorf("plan validator: %w", lerr)
				}
				if verr := core.ValidatePlan(p); verr != nil {
					// Restore previous value and re-save so the file stays consistent.
					if hadPrev {
						a.FrontMatter[field] = prev
					} else {
						delete(a.FrontMatter, field)
					}
					_ = a.Save()
					return verr
				}
			}

			return nil
		},
	}
}
