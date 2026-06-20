package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newSetCmd() *cobra.Command {
	var (
		flagAdd           bool
		flagRemove        []string
		flagAddSet        bool
		flagRemSet        bool
		flagAllowNewFacet []string
		flagJSON          bool
		flagCommand       string
		flagExpected      string
	)

	cmd := &cobra.Command{
		Use:   "set <type> <id> <field> [<value>...]",
		Short: "Set a frontmatter field on a vault artifact",
		Long: "Set a frontmatter field on a vault artifact.\n\n" +
			"Passing an empty string for an OPTIONAL scalar field (e.g. milestone, owner) " +
			"unsets it: the key is deleted rather than written as an empty value. " +
			"The JSON envelope reports status \"unset\" with to:null. " +
			"Required fields reject an empty value (minLength:1) as before.",
		Args: namedArgs("anvil set <type> <id> <field> [<value>...]", []string{"<type>", "<id>", "<field>"}, 3, -1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			id := args[1]
			// Canonicalise issue args through the same helper as the show read
			// path so write and read accept identical forms (qualified
			// "issue."-prefix, project-qualified ordinal, bare ordinal).
			if t == core.TypeIssue {
				id = core.ResolveIssueArg(v, id)
			}

			path := filepath.Join(v.Root, t.Dir(), id+".md")
			a, err := core.LoadArtifact(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: %s", ErrArtifactNotFound, id)
				}
				return fmt.Errorf("loading artifact: %w", err)
			}

			field := args[2]
			values := args[3:]

			if flagAddSet && flagRemSet {
				return fmt.Errorf("--add and --remove are mutually exclusive")
			}
			if flagRemSet && len(values) > 0 {
				return fmt.Errorf("positional values cannot be combined with --remove")
			}

			kind, err := schema.FieldKind(string(t), field)
			if err != nil {
				return fmt.Errorf("schema lookup: %w", err)
			}

			prev, hadPrev := a.FrontMatter[field]

			result := setResult{ID: id, Path: path, Field: field, Status: "set"}
			// fieldUnset is set to true when the caller passes an empty string for
			// an OPTIONAL scalar field, meaning "remove this key". ValidateField is
			// skipped in that path because the field is absent, not invalid.
			var fieldUnset bool

			switch kind {
			case schema.KindScalar:
				if flagAddSet || flagRemSet {
					return fmt.Errorf("%q is a scalar; pass a single positional value", field)
				}
				if len(values) != 1 {
					return fmt.Errorf("%q is a scalar; expected exactly 1 value, got %d", field, len(values))
				}
				v := values[0]
				// Empty string on an OPTIONAL scalar field means "unset": delete
				// the key rather than writing a malformed or empty value. Required
				// fields fall through so ValidateField's minLength:1 rejects "" as
				// before — deleting a required key would write a schema-invalid
				// artifact and skip validation. Check before normalization so
				// "milestone" wrapping never runs on an empty slug.
				if v == "" {
					required, rerr := schema.FieldRequired(string(t), field)
					if rerr != nil {
						return fmt.Errorf("schema lookup: %w", rerr)
					}
					if !required {
						delete(a.FrontMatter, field)
						result.From = prev
						result.To = nil
						result.Status = "unset"
						fieldUnset = true
						break
					}
				}
				if field == "milestone" {
					v = normalizeMilestone(v)
				}
				a.FrontMatter[field] = v
				result.From = prev
				result.To = v

			case schema.KindArray:
				switch {
				case flagAddSet:
					if len(values) == 0 {
						return fmt.Errorf("--add requires at least one positional value, e.g. anvil set %s %s %s --add foo bar", args[0], args[1], field)
					}
					existing := arrayValue(a.FrontMatter[field])
					before := append([]any(nil), existing...)
					next := make([]any, len(existing), len(existing)+len(values))
					copy(next, existing)
					added := make([]any, 0, len(values))
					for _, v := range values {
						next = append(next, v)
						added = append(added, v)
					}
					a.FrontMatter[field] = next
					result.From = before
					result.To = append([]any(nil), next...)
					result.Value = singleOrSlice(added)
					result.Status = "added"
				case len(values) > 0:
					// Bare positionals replace the array; use --add to append.
					before := arrayValue(a.FrontMatter[field])
					next := make([]any, len(values))
					for i, v := range values {
						next[i] = v
					}
					a.FrontMatter[field] = next
					result.From = before
					result.To = append([]any(nil), next...)
					// Value mirrors To so callers can assert to==value as a replace-vs-append signal.
					result.Value = append([]any(nil), next...)
				case flagRemSet:
					existing := arrayValue(a.FrontMatter[field])
					// Resolve every --remove target to an index in the original
					// slice, then splice in descending order so earlier indices
					// remain valid. Refuse duplicates / overlapping targets.
					indices := make([]int, 0, len(flagRemove))
					seen := make(map[int]bool, len(flagRemove))
					removed := make([]any, 0, len(flagRemove))
					for _, target := range flagRemove {
						idx := -1
						hadStringMatch := false
						for i, v := range existing {
							if s, ok := v.(string); ok && s == target {
								hadStringMatch = true
								if !seen[i] {
									idx = i
									break
								}
							}
						}
						if idx < 0 {
							if hadStringMatch {
								return fmt.Errorf("--remove %q: value already targeted in this invocation", target)
							}
							n, err := strconv.Atoi(target)
							if err != nil {
								return fmt.Errorf(
									"--remove %q: no such value in %q and not a valid 0-based index (len=%d); pass an existing value or an integer index",
									target, field, len(existing),
								)
							}
							if n < 0 || n >= len(existing) {
								return fmt.Errorf("--remove %d out of bounds (len=%d)", n, len(existing))
							}
							if seen[n] {
								return fmt.Errorf("--remove %d: index already targeted in this invocation", n)
							}
							idx = n
						}
						seen[idx] = true
						indices = append(indices, idx)
						removed = append(removed, existing[idx])
					}
					before := append([]any(nil), existing...)
					next := append([]any(nil), existing...)
					sort.Sort(sort.Reverse(sort.IntSlice(indices)))
					for _, idx := range indices {
						next = append(next[:idx], next[idx+1:]...)
					}
					a.FrontMatter[field] = next
					result.From = before
					result.To = append([]any(nil), next...)
					result.Value = singleOrSlice(removed)
					result.Status = "removed"
				default:
					return fmt.Errorf(
						"field %q is an array (field_is_array); pass values to replace, --add to append, or --remove\n"+
							"  replace: anvil set %s %s %s <value>...\n"+
							"  append:  anvil set %s %s %s <value>... --add\n"+
							"  remove:  anvil set %s %s %s --remove VALUE_OR_INDEX",
						field,
						args[0], args[1], field,
						args[0], args[1], field,
						args[0], args[1], field,
					)
				}

			case schema.KindObject:
				// reproduction_anchor is the only object field with a CLI authoring path.
				// --command is required; --expected defaults to empty string (records
				// the command without asserting output, useful during initial triage).
				if field != "reproduction_anchor" {
					return fmt.Errorf("%q is an object; edit the file directly", field)
				}
				if !cmd.Flags().Changed("command") {
					return fmt.Errorf("reproduction_anchor requires --command (and optionally --expected)")
				}
				if len(values) > 0 {
					return fmt.Errorf("reproduction_anchor does not accept positional values; use --command and --expected")
				}
				anchor := map[string]any{
					"command":  flagCommand,
					"expected": flagExpected,
				}
				result.From = prev
				result.To = anchor
				a.FrontMatter[field] = anchor

			case schema.KindUnknown:
				return fmt.Errorf("%q is not a declared field on %s; set only accepts schema-declared fields", field, t)
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

			if !fieldUnset {
				if err := schema.ValidateField(string(t), field, a.FrontMatter[field]); err != nil {
					return renderSchemaErr(cmd, v, path, err, flagJSON)
				}
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
				return fmt.Errorf("indexing %s: %w", id, err)
			}
			return emitSetResult(cmd, flagJSON, result)
		},
	}

	cmd.Flags().BoolVar(&flagAdd, "add", false, "signal append intent for an array field; values come from positional <value>... args")
	cmd.Flags().StringArrayVar(&flagRemove, "remove", nil, "remove a value from an array field; matches exact value, falls back to 0-based index (repeatable)")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet(s) to suppress novelty gate for (tags only)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.Flags().StringVar(&flagCommand, "command", "", "shell command for reproduction_anchor")
	cmd.Flags().StringVar(&flagExpected, "expected", "", "expected stdout for reproduction_anchor (empty = not asserted)")
	cmd.PreRunE = func(c *cobra.Command, _ []string) error {
		flagAddSet = c.Flags().Changed("add")
		flagRemSet = c.Flags().Changed("remove")
		return nil
	}

	return cmd
}

type setResult struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
	Value any    `json:"value,omitempty"`
	// Status is one of: set, added, removed, unset. "unset" (clearing an
	// optional scalar via empty string) carries to:null.
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

// singleOrSlice unwraps a one-element slice so single --add / --remove calls
// keep emitting their value as a bare scalar (preserves the JSON envelope
// shape callers built tooling against).
func singleOrSlice(v []any) any {
	if len(v) == 1 {
		return v[0]
	}
	return v
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
