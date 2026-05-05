package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newValidateCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "validate [path]",
		Short:   "Validate vault frontmatter against schemas",
		Args:    cobra.MaximumNArgs(1),
		Example: "  anvil validate\n  anvil validate --json\n  anvil validate /path/to/vault",
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

			var failures []*errfmt.ValidationError
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
						failures = append(failures, errfmt.NewValidationError("parse_error", path, "", err.Error()))
						continue
					}
					if err := schema.Validate(string(t), a.FrontMatter); err != nil {
						failures = append(failures, schemaErrToValidationErrors(path, err)...)
						continue
					}
					if t == core.TypeLearning {
						for _, vErr := range core.ValidateLearning(a, known) {
							failures = append(failures, errfmt.NewValidationError("constraint_violation", path, "", vErr.Error()))
						}
					}
				}
			}

			if asJSON {
				if failures == nil {
					failures = []*errfmt.ValidationError{}
				}
				b, _ := json.Marshal(failures)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				for i, f := range failures {
					if i > 0 {
						cmd.PrintErrln("")
					}
					cmd.PrintErrln(fmt.Sprintf("[%s] %s", f.Code, f.Path))
					if f.Field != "" {
						cmd.PrintErrln(fmt.Sprintf("  field: %s", f.Field))
					}
					if f.Got != "" {
						cmd.PrintErrln(fmt.Sprintf("  got: %s", f.Got))
					}
					if f.Expected != nil {
						cmd.PrintErrln(fmt.Sprintf("  expected: %v", f.Expected))
					}
					if f.Fix != "" {
						cmd.PrintErrln(fmt.Sprintf("  fix: %s", f.Fix))
					}
				}
			}

			if len(failures) > 0 {
				return ErrSchemaInvalid
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of structured errors")
	return cmd
}

// schemaErrToValidationErrors walks the validation tree and collects one
// ValidationError per leaf diagnostic.
func schemaErrToValidationErrors(path string, err error) []*errfmt.ValidationError {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []*errfmt.ValidationError{
			errfmt.NewValidationError("constraint_violation", path, "", err.Error()),
		}
	}
	var out []*errfmt.ValidationError
	walkSchemaErr(path, ve, &out)
	return out
}

func walkSchemaErr(path string, ve *jsonschema.ValidationError, out *[]*errfmt.ValidationError) {
	if len(ve.Causes) > 0 {
		for _, c := range ve.Causes {
			walkSchemaErr(path, c, out)
		}
		return
	}
	field := strings.Join(ve.InstanceLocation, ".")
	switch k := ve.ErrorKind.(type) {
	case *kind.Required:
		for _, name := range k.Missing {
			*out = append(*out, errfmt.NewValidationError("missing_required", path, name, ""))
		}
	case *kind.Enum:
		e := errfmt.NewValidationError("enum_violation", path, field, fmt.Sprint(k.Got))
		wantStrs := make([]string, 0, len(k.Want))
		for _, w := range k.Want {
			wantStrs = append(wantStrs, fmt.Sprint(w))
		}
		e.WithExpected(wantStrs)
		*out = append(*out, e)
	case *kind.Const:
		*out = append(*out, errfmt.NewValidationError("enum_violation", path, field, fmt.Sprint(k.Got)).
			WithExpected([]string{fmt.Sprint(k.Want)}))
	case *kind.Type:
		*out = append(*out, errfmt.NewValidationError("type_mismatch", path, field, k.Got).
			WithExpected(k.Want))
	case *kind.MinLength:
		*out = append(*out, errfmt.NewValidationError("constraint_violation", path, field, fmt.Sprintf("%d chars", k.Got)).
			WithExpected(fmt.Sprintf("min %d chars", k.Want)))
	case *kind.MaxLength:
		*out = append(*out, errfmt.NewValidationError("constraint_violation", path, field, fmt.Sprintf("%d chars", k.Got)).
			WithExpected(fmt.Sprintf("max %d chars", k.Want)))
	case *kind.Pattern:
		*out = append(*out, errfmt.NewValidationError("constraint_violation", path, field, k.Got).
			WithExpected(fmt.Sprintf("matches pattern %s", k.Want)))
	case *kind.Format:
		*out = append(*out, errfmt.NewValidationError("constraint_violation", path, field, fmt.Sprint(k.Got)).
			WithExpected(fmt.Sprintf("format %s", k.Want)))
	case *kind.AdditionalProperties:
		for _, prop := range k.Properties {
			*out = append(*out, errfmt.NewValidationError("constraint_violation", path, prop, "unexpected").
				WithExpected("not present"))
		}
	default:
		*out = append(*out, errfmt.NewValidationError("constraint_violation", path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	}
}
