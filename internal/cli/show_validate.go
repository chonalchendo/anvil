package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

// runShowValidate handles `anvil show <type> <id> --validate` for any
// type. Schema and link-resolution errors aggregate: both are reported, and the returned
// error is non-nil if either fails.
func runShowValidate(cmd *cobra.Command, v *core.Vault, t core.Type, id string, asJSON bool) error {
	path := filepath.Join(v.Root, t.Dir(), id+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrArtifactNotFound
		}
		return fmt.Errorf("loading artifact: %w", err)
	}

	schemaErr := schema.Validate(string(t), a.FrontMatter)
	links := core.ResolveLinks(v, a.FrontMatter)
	// Scoped to data-model: validateOne's other body checks would newly fail
	// the existing back catalogue of other types under show --validate.
	var bodyErrs []error
	if t == core.TypeDataModel {
		bodyErrs = core.ValidateDataModel(a)
	}

	if asJSON {
		out := map[string]any{
			"schema_ok":        schemaErr == nil,
			"unresolved_links": links,
		}
		if len(bodyErrs) > 0 {
			msgs := make([]string, len(bodyErrs))
			for i, e := range bodyErrs {
				msgs[i] = e.Error()
			}
			out["body_errors"] = msgs
		}
		if schemaErr != nil {
			out["schema_errors"] = []string{schemaErr.Error()}
		}
		b, _ := json.Marshal(out)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
	} else {
		emitFrontMatterText(cmd, a.FrontMatter)
		if schemaErr != nil {
			cmd.PrintErrln("schema: fail:", schemaErr)
		} else {
			cmd.PrintErrln("schema: ok")
		}
		for _, e := range bodyErrs {
			cmd.PrintErrln("body: fail:", e)
		}
		if len(links) > 0 {
			cmd.PrintErrln("links:")
			for _, l := range links {
				cmd.PrintErrf("  - %s [[%s]]: not found\n", l.Field, l.Target)
			}
		} else {
			cmd.PrintErrln("links: ok")
		}
	}

	switch {
	case schemaErr != nil:
		return fmt.Errorf("%w: %w", ErrSchemaInvalid, schemaErr)
	case len(bodyErrs) > 0:
		return fmt.Errorf("%w: %w", ErrSchemaInvalid, bodyErrs[0])
	case len(links) > 0:
		return fmt.Errorf("%w: %d unresolved", ErrUnresolvedLinks, len(links))
	}
	return nil
}
