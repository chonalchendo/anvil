package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newSetCmd() *cobra.Command {
	var (
		flagAdd           string
		flagRemove        int
		flagAddSet        bool
		flagRemSet        bool
		flagAllowNewFacet []string
		flagJSON          bool
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

			result := setResult{ID: args[1], Path: path, Field: field, Status: "set"}

			switch kind {
			case schema.KindScalar:
				if flagAddSet || flagRemSet {
					return fmt.Errorf("%q is a scalar; pass a single positional value", field)
				}
				if len(values) != 1 {
					return fmt.Errorf("%q is a scalar; expected exactly 1 value, got %d", field, len(values))
				}
				a.FrontMatter[field] = values[0]
				result.From = prev
				result.To = values[0]

			case schema.KindArray:
				switch {
				case flagAddSet:
					existing := arrayValue(a.FrontMatter[field])
					before := append([]any(nil), existing...)
					next := append(existing, flagAdd)
					a.FrontMatter[field] = next
					result.From = before
					result.To = append([]any(nil), next...)
					result.Value = flagAdd
					result.Status = "added"
				case flagRemSet:
					existing := arrayValue(a.FrontMatter[field])
					if flagRemove < 0 || flagRemove >= len(existing) {
						return fmt.Errorf("--remove %d out of bounds (len=%d)", flagRemove, len(existing))
					}
					before := append([]any(nil), existing...)
					removed := existing[flagRemove]
					next := append(existing[:flagRemove], existing[flagRemove+1:]...)
					a.FrontMatter[field] = next
					result.From = before
					result.To = append([]any(nil), next...)
					result.Value = removed
					result.Status = "removed"
				default:
					sample := "VALUE"
					if len(values) > 0 {
						sample = values[0]
					}
					return fmt.Errorf(
						"field %q is an array (field_is_array); positional values are not accepted — use --add or --remove\n"+
							"  corrected: anvil set %s %s %s --add %q\n"+
							"             anvil set %s %s %s --remove INDEX",
						field,
						args[0], args[1], field, sample,
						args[0], args[1], field,
					)
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
				result.From = prev
				result.To = values[0]
			}

			if field == "tags" {
				for _, f := range flagAllowNewFacet {
					if !facets.Has(f) {
						return formatEnumError("--allow-new-facet", f, facets.Names(), "")
					}
				}
				allowed := map[string]bool{}
				for _, f := range flagAllowNewFacet {
					allowed[f] = true
				}
				vaultValues, skipped, vErr := facets.CollectValues(v.Root)
				if vErr != nil {
					return fmt.Errorf("walking vault: %w", vErr)
				}
				for _, p := range skipped {
					cmd.PrintErrln("warn: skipped corrupt artifact during facet walk: " + p)
				}
				tagsRaw, _ := a.FrontMatter[field].([]any)
				tagsStr := make([]string, 0, len(tagsRaw))
				for _, raw := range tagsRaw {
					if s, ok := raw.(string); ok {
						tagsStr = append(tagsStr, s)
					}
				}
				if errs := facets.Check(vaultValues, tagsStr, allowed); len(errs) > 0 {
					for _, e := range errs {
						e.Path = path
					}
					emitValidationErrors(cmd, flagJSON, errs)
					return ErrSchemaInvalid
				}
			}

			if err := schema.Validate(string(t), a.FrontMatter); err != nil {
				return renderSchemaErr(cmd, v, path, err, flagJSON)
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
			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("indexing %s: %w", args[1], err)
			}
			return emitSetResult(cmd, flagJSON, result)
		},
	}

	cmd.Flags().StringVar(&flagAdd, "add", "", "append a value to an array field")
	cmd.Flags().IntVar(&flagRemove, "remove", 0, "remove the value at the given 0-based index from an array field")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet(s) to suppress novelty gate for (tags only)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.PreRunE = func(c *cobra.Command, _ []string) error {
		flagAddSet = c.Flags().Changed("add")
		flagRemSet = c.Flags().Changed("remove")
		return nil
	}

	return cmd
}

type setResult struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Field  string `json:"field"`
	From   any    `json:"from"`
	To     any    `json:"to"`
	Value  any    `json:"value,omitempty"`
	Status string `json:"status"`
}

func emitSetResult(cmd *cobra.Command, asJSON bool, r setResult) error {
	if asJSON {
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	switch r.Status {
	case "added":
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s + %s\n", r.ID, r.Field, formatSetValue(r.Value))
	case "removed":
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s − %s\n", r.ID, r.Field, formatSetValue(r.Value))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s → %s\n", r.ID, r.Field, formatSetValue(r.From), formatSetValue(r.To))
	}
	return nil
}

func formatSetValue(v any) string {
	if v == nil {
		return "<unset>"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
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
