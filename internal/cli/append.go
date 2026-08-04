package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// newAppendCmd wires the append verb: the only CLI route to grow an
// artifact's body after creation. Session addenda, design reconciliation
// notes, and contract precedents all land this way instead of a raw file
// edit — which bypasses the body validation `create` enforces and silently
// skips the `updated` bump. Append reuses both: the same
// validateBeforeCreate layer create runs (wikilink resolution, per-type
// structural checks), and the same load/mutate/save/index sequence set and
// rename use.
func newAppendCmd() *cobra.Command {
	var (
		flagBody          string
		flagBodyFile      string
		flagJSON          bool
		flagAllowNewFacet []string
	)

	cmd := &cobra.Command{
		Use:   "append <type> <id> --body-file <f>",
		Short: "Append a validated body section to a vault artifact, bumping updated",
		Long: "Append a Markdown section to an artifact's body and save it.\n\n" +
			"The appended content is validated through the same checks `anvil create` " +
			"runs against an authored body (wikilink resolution, per-type structural " +
			"checks) before anything is written, and `updated` is bumped to today. " +
			"This is append-only — replacing, deleting, or reordering existing " +
			"sections is not supported; edit the file directly for that.",
		Args: namedArgs("anvil append <type> <id> --body-file <f>", []string{"<type>", "<id>"}, 2, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			id, path := core.ResolveArtifact(v, t, args[1])
			a, err := core.LoadArtifact(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: %s", ErrArtifactNotFound, id)
				}
				return fmt.Errorf("loading artifact: %w", err)
			}

			addition, err := readBody(cmd, flagBody, flagBodyFile)
			if err != nil {
				return err
			}
			if addition == "" {
				return fmt.Errorf("no content to append; pass --body or --body-file")
			}

			newBody := joinBodySection(a.Body, addition)
			fm := a.FrontMatter
			fm["updated"] = time.Now().UTC().Format("2006-01-02")

			if err := validateBeforeCreate(cmd, v, t, path, fm, newBody, true, flagAllowNewFacet, flagJSON); err != nil {
				return err
			}

			a.Body = newBody
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("indexing %s: %w", id, err)
			}

			return emitAppendResult(cmd, flagJSON, appendResult{
				ID: id, Path: path, Updated: fm["updated"].(string),
			})
		},
	}

	cmd.Flags().StringVar(&flagBody, "body", "", "section content to append (literal, or \"-\" for stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read section content to append from a file")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet(s) to suppress novelty gate for (tags only)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	return cmd
}

// joinBodySection appends addition to existing, separated by exactly one
// blank line, so a new H2 section never runs into the previous one's last
// line regardless of whether existing already ends in trailing newlines.
func joinBodySection(existing, addition string) string {
	trimmed := strings.TrimRight(existing, "\n")
	if trimmed == "" {
		return addition
	}
	return trimmed + "\n\n" + addition
}

type appendResult struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Updated string `json:"updated"`
}

func emitAppendResult(cmd *cobra.Command, asJSON bool, r appendResult) error {
	if asJSON {
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: appended (updated %s)\n", r.ID, r.Updated)
	return nil
}
