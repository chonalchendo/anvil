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
// artifact's body after creation. Session addenda and design reconciliation
// notes all land this way instead of a raw file
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
			"is a no-op. Only blocking findings the append introduces refuse it; " +
			"warnings and errors already present in the stored body never block.\n\n" +
			"Warnings: advisory findings never fail append. Under --json they ride the " +
			"success envelope's `warnings` array as {kind:\"validation\", code, got}; in " +
			"text mode they print to stderr. Pre-existing errors surface the same way, " +
			"prefixed \"pre-existing (not introduced by this append)\".\n\n" +
			"This is append-only — replacing, deleting, or reordering " +
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

			res, err := appendBodyCore(cmd, v, t, path, a, addition)
			if err != nil {
				return err
			}
			if res.status == "unchanged" {
				return emitAppendResult(cmd, flagJSON, appendResult{
					ID: id, Path: path, Status: "unchanged",
				})
			}
			if res.blocked {
				return emitValidationErrors(cmd, flagJSON, res.failures)
			}

			return emitAppendResult(cmd, flagJSON, appendResult{
				ID: id, Path: path, Updated: a.FrontMatter["updated"].(string), Status: "appended",
				findings: res.failures,
			})
		},
	}

	cmd.Flags().StringVar(&flagBody, "body", "", "section content to append (literal, or \"-\" for stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read section content to append from a file")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	return cmd
}

type appendCoreResult struct {
	status   string // "appended" or "unchanged"
	blocked  bool
	failures []*errfmt.ValidationError
}

// appendBodyCore is the write path anvil append and anvil verify --replay
// share: the retry-safety no-op, the introduced-failure validation, the
// `updated` bump and the atomic swap. A blocked result writes nothing.
func appendBodyCore(cmd *cobra.Command, v *core.Vault, t core.Type, path string, a *core.Artifact, addition string) (appendCoreResult, error) {
	// Retry safety: an agent re-running an append whose response was
	// lost must not duplicate the section. The stored body ends with
	// exactly the addition after a successful run, so a suffix match
	// is the already-applied signal — no write, no updated bump.
	if strings.HasSuffix(a.Body, addition) {
		return appendCoreResult{status: "unchanged"}, nil
	}
	newBody := joinBodySection(a.Body, addition)
	failures := staticBodyFailures(cmd, v, t, path, a.FrontMatter, newBody)
	var introduced []*errfmt.ValidationError
	if len(failures) > 0 {
		introduced = markPreexisting(failures, staticBodyFailures(cmd, v, t, path, a.FrontMatter, a.Body))
	}
	// An append never edits existing content, so only blocking findings
	// it introduced refuse. Warnings and pre-existing errors ride out
	// with the success result instead of dropping the section.
	if hasBlockingFailure(introduced) {
		return appendCoreResult{blocked: true, failures: failures}, nil
	}
	a.Body = newBody
	a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")
	// yaml.v3 loads YYYY-MM-DD scalars as time.Time and would re-emit
	// them as full timestamps; append rewrites frontmatter it didn't
	// author, so normalise before marshalling.
	normaliseDates(a.FrontMatter)
	content, err := a.Marshal()
	if err != nil {
		return appendCoreResult{}, fmt.Errorf("marshalling %s: %w", path, err)
	}
	// atomicSwap, not a truncating write: the file holds content this
	// command didn't author, and an interrupted rewrite must never be
	// able to destroy it.
	if err := atomicSwap(path, path, content); err != nil {
		return appendCoreResult{}, fmt.Errorf("saving artifact: %w", err)
	}
	if err := indexAfterSave(v, a); err != nil {
		return appendCoreResult{}, fmt.Errorf("indexing %s: %w", path, err)
	}
	return appendCoreResult{status: "appended", failures: failures}, nil
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
// exhibits on its own and returns the rest — the ones the addendum
// introduced. The combined body is what gets validated, but an append never
// edits existing content, so only introduced failures may refuse it.
func markPreexisting(failures, old []*errfmt.ValidationError) (introduced []*errfmt.ValidationError) {
	seen := make(map[string]bool, len(old))
	for _, e := range old {
		seen[e.Code+"\x00"+e.Field+"\x00"+e.Got] = true
	}
	for _, e := range failures {
		if seen[e.Code+"\x00"+e.Field+"\x00"+e.Got] {
			e.Got = "pre-existing (not introduced by this append): " + e.Got
			continue
		}
		introduced = append(introduced, e)
	}
	return introduced
}

type appendResult struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Updated string `json:"updated,omitempty"`
	Status  string `json:"status"`
	// Warnings mirrors create's envelope, built from findings at emit time.
	Warnings []map[string]string `json:"warnings,omitempty"`
	findings []*errfmt.ValidationError
}

func emitAppendResult(cmd *cobra.Command, asJSON bool, r appendResult) error {
	if asJSON {
		r.Warnings = jsonWarnings(nil, r.findings)
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	if r.Status == "unchanged" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: unchanged (section already present)\n", r.ID)
		return nil
	}
	if len(r.findings) > 0 {
		printValidationErrors(cmd, r.findings)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: appended (updated %s)\n", r.ID, r.Updated)
	return nil
}
