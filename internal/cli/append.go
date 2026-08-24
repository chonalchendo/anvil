package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// newAppendCmd wires the append verb: the only CLI route to grow an
// artifact's body after creation. Session addenda, design reconciliation
// notes, and contract precedents all land this way instead of a raw file
// edit — which bypasses the body validation `create` enforces and silently
// skips the `updated` bump. Appended content runs the same static body
// checks create runs (wikilink resolution, per-type structural checks) via
// staticBodyFailures — but never the create-time feasibility gate: an
// issue's Verification blocks assert the tree's pre-fix state at authoring
// time, so executing them here would refuse the append on every
// already-fixed issue and run arbitrary bash for what is a body edit.
func newAppendCmd() *cobra.Command {
	var (
		flagBody     string
		flagBodyFile string
		flagJSON     bool
	)

	cmd := &cobra.Command{
		Use:   "append <type> <id> --body-file <f>",
		Short: "Append a validated body section to a vault artifact, bumping updated",
		Long: "Append a Markdown section to an artifact's body and save it.\n\n" +
			"The appended content is validated through the same static checks `anvil create` " +
			"runs against an authored body (wikilink resolution, per-type structural " +
			"checks) before anything is written, and `updated` is bumped to today. " +
			"Verification blocks are never executed. Re-running an identical append " +
			"is a no-op. This is append-only — replacing, deleting, or reordering " +
			"existing sections is not supported; edit the file directly for that.",
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

			id, path, err := core.ResolveArtifact(v, t, args[1])
			if err != nil {
				return err
			}
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
				if flagBodyFile != "" {
					return fmt.Errorf("--body-file %s is empty; write the section content to it or pass --body \"<text>\"", flagBodyFile)
				}
				return fmt.Errorf("no content to append; pass --body or --body-file")
			}

			// Retry safety: an agent re-running an append whose response was
			// lost must not duplicate the section. The stored body ends with
			// exactly the addition after a successful run, so a suffix match
			// is the already-applied signal — no write, no updated bump.
			if strings.HasSuffix(a.Body, addition) {
				return emitAppendResult(cmd, flagJSON, appendResult{
					ID: id, Path: path, Status: "unchanged",
				})
			}

			newBody := joinBodySection(a.Body, addition)
			if failures := staticBodyFailures(cmd, v, t, path, a.FrontMatter, newBody); len(failures) > 0 {
				markPreexisting(failures, staticBodyFailures(cmd, v, t, path, a.FrontMatter, a.Body))
				return emitValidationErrors(cmd, flagJSON, failures)
			}

			a.Body = newBody
			a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
			// yaml.v3 loads YYYY-MM-DD scalars as time.Time and would re-emit
			// them as full timestamps; append rewrites frontmatter it didn't
			// author, so normalise before marshalling.
			normaliseDates(a.FrontMatter)
			content, err := a.Marshal()
			if err != nil {
				return fmt.Errorf("marshalling %s: %w", id, err)
			}
			// atomicSwap, not a truncating write: the file holds content this
			// command didn't author, and an interrupted rewrite must never be
			// able to destroy it.
			if err := atomicSwap(path, path, content); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("indexing %s: %w", id, err)
			}

			return emitAppendResult(cmd, flagJSON, appendResult{
				ID: id, Path: path, Updated: a.FrontMatter["updated"].(string), Status: "appended",
			})
		},
	}

	cmd.Flags().StringVar(&flagBody, "body", "", "section content to append (literal, or \"-\" for stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read section content to append from a file")
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

// markPreexisting prefixes each failure that the stored body already
// exhibits on its own, so a refusal names honestly which violations the
// addendum introduced and which it merely re-surfaces — the combined body is
// what gets validated, but an append never edits existing content, and an
// error citing content the author never touched otherwise reads as a bug.
func markPreexisting(failures, old []*errfmt.ValidationError) {
	seen := make(map[string]bool, len(old))
	for _, e := range old {
		seen[e.Code+"\x00"+e.Field+"\x00"+e.Got] = true
	}
	for _, e := range failures {
		if seen[e.Code+"\x00"+e.Field+"\x00"+e.Got] {
			e.Got = "pre-existing (not introduced by this append): " + e.Got
		}
	}
}

type appendResult struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Updated string `json:"updated,omitempty"`
	Status  string `json:"status"`
}

func emitAppendResult(cmd *cobra.Command, asJSON bool, r appendResult) error {
	if asJSON {
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	if r.Status == "unchanged" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: unchanged (section already present)\n", r.ID)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: appended (updated %s)\n", r.ID, r.Updated)
	return nil
}
