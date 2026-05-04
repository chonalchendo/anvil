package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newSetCmd() *cobra.Command {
	var (
		flagAdd    string
		flagRemove int
		flagAddSet bool
		flagRemSet bool
	)

	cmd := &cobra.Command{
		Use:   "set <type> <id> <field> <value>...",
		Short: "Set a frontmatter field on a vault artifact",
		Args:  cobra.MinimumNArgs(3),
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

			field := args[2]
			values := args[3:]

			if flagAddSet && flagRemSet {
				return fmt.Errorf("--add and --remove are mutually exclusive")
			}
			if (flagAddSet || flagRemSet) && len(values) > 0 {
				return fmt.Errorf("positional values cannot be combined with --add or --remove")
			}

			kind, err := schema.FieldKind(string(t), field)
			if err != nil {
				return fmt.Errorf("schema lookup: %w", err)
			}

			prev, hadPrev := a.FrontMatter[field]

			switch kind {
			case schema.KindScalar:
				if flagAddSet || flagRemSet {
					return fmt.Errorf("%q is a scalar; pass a single positional value", field)
				}
				if len(values) != 1 {
					return fmt.Errorf("%q is a scalar; expected exactly 1 value, got %d", field, len(values))
				}
				a.FrontMatter[field] = values[0]

			case schema.KindArray:
				switch {
				case flagAddSet:
					existing := arrayValue(a.FrontMatter[field])
					a.FrontMatter[field] = append(existing, flagAdd)
				case flagRemSet:
					existing := arrayValue(a.FrontMatter[field])
					if flagRemove < 0 || flagRemove >= len(existing) {
						return fmt.Errorf("--remove %d out of bounds (len=%d)", flagRemove, len(existing))
					}
					a.FrontMatter[field] = append(existing[:flagRemove], existing[flagRemove+1:]...)
				default:
					if len(values) == 0 {
						return fmt.Errorf("%q is an array; pass values as positional args, --add, or --remove", field)
					}
					out := make([]any, len(values))
					for i, s := range values {
						out[i] = s
					}
					a.FrontMatter[field] = out
				}

			case schema.KindObject:
				return fmt.Errorf("%q is an object; edit the file directly", field)

			case schema.KindUnknown:
				if flagAddSet || flagRemSet {
					return fmt.Errorf("%q is not a known field; cannot use --add/--remove", field)
				}
				if len(values) != 1 {
					return fmt.Errorf("%q is not a known field; expected exactly 1 value, got %d", field, len(values))
				}
				a.FrontMatter[field] = values[0]
			}

			if err := schema.Validate(string(t), a.FrontMatter); err != nil {
				return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
			}
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}

			if t == core.TypePlan && field == "status" && len(values) == 1 && values[0] == "locked" {
				p, lerr := core.LoadPlan(a.Path)
				if lerr != nil {
					return fmt.Errorf("plan validator: %w", lerr)
				}
				if verr := core.ValidatePlan(p); verr != nil {
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

	cmd.Flags().StringVar(&flagAdd, "add", "", "append a value to an array field")
	cmd.Flags().IntVar(&flagRemove, "remove", 0, "remove the value at the given 0-based index from an array field")
	cmd.PreRunE = func(c *cobra.Command, _ []string) error {
		flagAddSet = c.Flags().Changed("add")
		flagRemSet = c.Flags().Changed("remove")
		return nil
	}

	return cmd
}

// arrayValue normalises a frontmatter value into []any. yaml.v3 may decode
// arrays as []any, []string, or nil (absent).
func arrayValue(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out
	}
	return nil
}
