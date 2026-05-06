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
						failures = append(failures, errfmt.NewValidationError(errfmt.CodeParseError, path, "", err.Error()))
						continue
					}
					if err := schema.Validate(string(t), a.FrontMatter); err != nil {
						failures = append(failures, schemaErrToValidationErrors(path, err)...)
						continue
					}
					if t == core.TypeLearning {
						for _, vErr := range core.ValidateLearning(a, known) {
							failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
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
				printValidationErrors(cmd, failures)
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
			errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", err.Error()),
		}
	}
	var out []*errfmt.ValidationError
	walkSchemaErr(path, ve, &out)
	return out
}

func walkSchemaErr(path string, ve *jsonschema.ValidationError, out *[]*errfmt.ValidationError) {
	// MinContains/Contains have causes (the failing pattern leaves), but we
	// want to emit one structured error at this level, not recurse into the
	// raw pattern failures — intercept before the generic cause-recurse.
	if _, ok := ve.ErrorKind.(*kind.MinContains); ok {
		field := strings.Join(ve.InstanceLocation, ".")
		if field == "tags" {
			pattern := "^domain/[a-z0-9-]+$" // fallback
			for _, c := range ve.Causes {
				if p, ok := c.ErrorKind.(*kind.Pattern); ok {
					pattern = p.Want
					break
				}
			}
			*out = append(*out, errfmt.NewValidationError(errfmt.CodeMissingRequiredFacet, path, "tags", "").
				WithExpected([]string{pattern}).
				WithFix("add a tag matching the listed pattern (e.g. domain/<x>)"))
			return
		}
	}
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
			*out = append(*out, errfmt.NewValidationError(errfmt.CodeMissingRequired, path, name, ""))
		}
	case *kind.Enum:
		e := errfmt.NewValidationError(errfmt.CodeEnumViolation, path, field, fmt.Sprint(k.Got))
		wantStrs := make([]string, 0, len(k.Want))
		for _, w := range k.Want {
			wantStrs = append(wantStrs, fmt.Sprint(w))
		}
		e.WithExpected(wantStrs)
		*out = append(*out, e)
	case *kind.Const:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeEnumViolation, path, field, fmt.Sprint(k.Got)).
			WithExpected([]string{fmt.Sprint(k.Want)}))
	case *kind.Type:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeTypeMismatch, path, field, k.Got).
			WithExpected(k.Want))
	case *kind.MinLength:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%d chars", k.Got)).
			WithExpected(fmt.Sprintf("min %d chars", k.Want)))
	case *kind.MaxLength:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%d chars", k.Got)).
			WithExpected(fmt.Sprintf("max %d chars", k.Want)))
	case *kind.Pattern:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, k.Got).
			WithExpected(fmt.Sprintf("matches pattern %s", k.Want)))
	case *kind.Format:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprint(k.Got)).
			WithExpected(fmt.Sprintf("format %s", k.Want)))
	case *kind.AdditionalProperties:
		for _, prop := range k.Properties {
			*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, prop, "unexpected").
				WithExpected("not present"))
		}
	case *kind.Contains:
		// MinContains is intercepted earlier; Contains may still arrive here
		// for the rare zero-cause path. On tags, treat it as a missing facet.
		if field == "tags" {
			pattern := "^domain/[a-z0-9-]+$" // fallback
			for _, c := range ve.Causes {
				if p, ok := c.ErrorKind.(*kind.Pattern); ok {
					pattern = p.Want
					break
				}
			}
			*out = append(*out, errfmt.NewValidationError(errfmt.CodeMissingRequiredFacet, path, "tags", "").
				WithExpected([]string{pattern}).
				WithFix("add a tag matching the listed pattern (e.g. domain/<x>)"))
			return
		}
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	default:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	}
}
