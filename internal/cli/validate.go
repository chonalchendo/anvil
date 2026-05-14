package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/schemas"
)

func newValidateCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "validate [path]",
		Short:   "Validate vault frontmatter against schemas",
		Args:    cobra.MaximumNArgs(1),
		Example: "  anvil validate\n  anvil validate --json\n  anvil validate /path/to/vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			var root, singleFile string
			if len(args) == 1 {
				fi, err := os.Stat(args[0])
				if err != nil {
					return fmt.Errorf("stat %s: %w", args[0], err)
				}
				if fi.IsDir() {
					root = args[0]
				} else {
					vaultRoot, err := vaultRootFromArtifactPath(args[0])
					if err != nil {
						return err
					}
					root = vaultRoot
					singleFile = args[0]
				}
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
			if singleFile != "" {
				t, err := typeFromArtifactPath(singleFile)
				if err != nil {
					return err
				}
				failures = validateOne(t, singleFile, known)
			} else {
				for _, t := range core.AllTypes {
					paths, err := collectArtifactPaths(root, t)
					if err != nil {
						return err
					}
					for _, p := range paths {
						failures = append(failures, validateOne(t, p, known)...)
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

// validateOne runs schema + learning-body checks on a single artifact file and
// returns any structured failures.
func validateOne(t core.Type, path string, knownTags map[string]struct{}) []*errfmt.ValidationError {
	a, err := core.LoadArtifact(path)
	if err != nil {
		return []*errfmt.ValidationError{errfmt.NewValidationError(errfmt.CodeParseError, path, "", err.Error())}
	}
	if err := schema.Validate(string(t), a.FrontMatter); err != nil {
		return schemaErrToValidationErrors(path, err)
	}

	var out []*errfmt.ValidationError

	if t == core.TypeLearning {
		// ValidateLearning covers both body-shape and glossary membership for
		// learnings; the generic drift check below skips learnings to avoid
		// double-reporting.
		for _, vErr := range core.ValidateLearning(a, knownTags) {
			out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
		}
		return out
	}

	if t == core.TypeIssue {
		for _, vErr := range core.ValidateIssue(a) {
			out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
		}
	}

	// Drift check: flag tags not present in the glossary. Skipped when the
	// glossary is empty so fresh vaults don't fail until any tags are defined.
	if knownTags != nil {
		raw, _ := a.FrontMatter["tags"].([]any)
		for _, item := range raw {
			tag, ok := item.(string)
			if !ok {
				continue
			}
			if _, _, valid := glossary.SplitTag(tag); !valid {
				// Malformed shape — schema layer surfaces these.
				continue
			}
			if _, defined := knownTags[tag]; !defined {
				out = append(out, errfmt.NewValidationError(errfmt.CodeUnknownGlossaryTag, path, "tags", tag).
					WithFix(fmt.Sprintf(`add it via "anvil tags add %s --desc \"...\""`, tag)))
			}
		}
	}

	return out
}

// vaultRootFromArtifactPath resolves the vault root for an artifact file by
// matching the parent directory name against the known type-dir set.
func vaultRootFromArtifactPath(path string) (string, error) {
	parent := filepath.Dir(path)
	for _, t := range core.AllTypes {
		if filepath.Base(parent) == t.Dir() {
			return filepath.Dir(parent), nil
		}
	}
	// Singletons (product-design, system-design) live at
	// <vault>/05-projects/<project>/<type>.md — one level deeper.
	if filepath.Base(filepath.Dir(parent)) == "05-projects" {
		return filepath.Dir(filepath.Dir(parent)), nil
	}
	return "", errfmt.NewNotInVault(path)
}

// typeFromArtifactPath infers the Type from the artifact's parent dir.
func typeFromArtifactPath(path string) (core.Type, error) {
	parent := filepath.Base(filepath.Dir(path))
	for _, t := range core.AllTypes {
		if t.Dir() == parent {
			return t, nil
		}
	}
	// Singleton case: parent is the project dir under 05-projects/.
	if filepath.Base(filepath.Dir(filepath.Dir(path))) == "05-projects" {
		stem := strings.TrimSuffix(filepath.Base(path), ".md")
		for _, t := range core.AllTypes {
			if string(t) == stem {
				return t, nil
			}
		}
	}
	return "", errfmt.NewNotInVault(path)
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
			pattern := tagsContainsPattern(ve)
			*out = append(*out, missingFacetErr(path, pattern))
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
			*out = append(*out, missingFacetErr(path, tagsContainsPattern(ve)))
			return
		}
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	default:
		*out = append(*out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, field, fmt.Sprintf("%v", ve.ErrorKind)))
	}
}

// tagsContainsPattern returns the pattern from the `contains` schema that this
// MinContains/Contains node enforces. With non-empty tags, the failing Pattern
// cause carries the pattern verbatim; with zero matching tags MinContains has
// no causes, so we resolve the pattern from the schema URL itself (which ends
// in `.../properties/tags/allOf/N` or `.../properties/tags`).
func tagsContainsPattern(ve *jsonschema.ValidationError) string {
	for _, c := range ve.Causes {
		if p, ok := c.ErrorKind.(*kind.Pattern); ok {
			return p.Want
		}
	}
	if p := patternFromSchemaURL(ve.SchemaURL); p != "" {
		return p
	}
	return "^domain/[a-z0-9-]+$"
}

// patternFromSchemaURL parses the fragment of a schema URL like
// `https://anvil.dev/schemas/<type>.schema.json#/properties/tags/allOf/N`
// and returns the contains-clause pattern at that location. Empty string on
// any parse failure — caller falls back to a default.
func patternFromSchemaURL(schemaURL string) string {
	hash := strings.Index(schemaURL, "#")
	if hash < 0 {
		return ""
	}
	resource := schemaURL[:hash]
	frag := schemaURL[hash+1:]
	const prefix = "https://anvil.dev/schemas/"
	if !strings.HasPrefix(resource, prefix) {
		return ""
	}
	name := strings.TrimPrefix(resource, prefix)
	raw, err := schemas.FS.ReadFile(name)
	if err != nil {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return ""
	}
	// resolve JSON pointer fragment manually — small alphabet, no need for a lib.
	parts := strings.Split(strings.TrimPrefix(frag, "/"), "/")
	var node any = root
	for _, p := range parts {
		if p == "" {
			continue
		}
		switch n := node.(type) {
		case map[string]any:
			node = n[p]
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= len(n) {
				return ""
			}
			node = n[idx]
		default:
			return ""
		}
	}
	// node may itself be the contains schema (allOf entry) — descend if so.
	if m, ok := node.(map[string]any); ok {
		if c, ok := m["contains"].(map[string]any); ok {
			node = c
		}
		if m2, ok := node.(map[string]any); ok {
			if p, ok := m2["pattern"].(string); ok {
				return p
			}
		}
	}
	return ""
}

// missingFacetErr builds the canonical missing_required_facet error for the
// given tags-pattern. The fix text is generic; create.go / set.go augment it
// with vault-aware hints (existing facet values, --allow-new-facet) before
// printing.
func missingFacetErr(path, pattern string) *errfmt.ValidationError {
	facet := facetNameFromPattern(pattern)
	example := fmt.Sprintf("e.g. %s/<x>", facet)
	if facet == "" {
		example = "e.g. domain/<x>"
	}
	return errfmt.NewValidationError(errfmt.CodeMissingRequiredFacet, path, "tags", "").
		WithExpected([]string{pattern}).
		WithFix(fmt.Sprintf("add a tag matching the listed pattern (%s)", example))
}

// facetNameFromPattern extracts the facet prefix (e.g. "domain", "activity")
// from a tags pattern like `^domain/[a-z0-9-]+$`. Empty string if the pattern
// doesn't follow that shape.
func facetNameFromPattern(pattern string) string {
	p := strings.TrimPrefix(pattern, "^")
	slash := strings.Index(p, "/")
	if slash <= 0 {
		return ""
	}
	return p[:slash]
}
