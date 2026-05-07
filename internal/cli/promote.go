package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newPromoteCmd() *cobra.Command {
	var (
		flagAs            string
		flagJSON          bool
		flagTags          []string
		flagAllowNewFacet []string
		flagProjectLocal  string
	)

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promote an inbox entry to a typed artifact",
		Long:  "Operates on inbox entries (the only promotable type today). --as selects the target type.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			validAs := []string{"issue", "thread", "learning", "discard"}
			valid := false
			for _, v := range validAs {
				if flagAs == v {
					valid = true
					break
				}
			}
			if !valid {
				return formatEnumError(
					"--as", flagAs, validAs,
					fmt.Sprintf("anvil promote %s --as issue", id),
				)
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			inboxPath := filepath.Join(v.Root, core.TypeInbox.Dir(), id+".md")
			a, err := core.LoadArtifact(inboxPath)
			if err != nil {
				if os.IsNotExist(err) {
					return ErrArtifactNotFound
				}
				return fmt.Errorf("loading inbox entry: %w", err)
			}

			switch flagAs {
			case "discard":
				return discardInbox(cmd, v, a, id, flagJSON)
			default:
				return promoteToTyped(cmd, v, a, id, core.Type(flagAs), flagJSON, flagTags, flagAllowNewFacet, flagProjectLocal)
			}
		},
	}

	cmd.Flags().StringVar(&flagAs, "as", "", "promotion target type (issue|thread|learning|discard)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "tags to seed on promoted artifact")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet(s) to suppress novelty gate for")
	cmd.Flags().StringVar(&flagProjectLocal, "project", "", "project slug for the promoted issue (overrides inbox suggested_project and resolver)")
	_ = cmd.MarkFlagRequired("as")
	return cmd
}

// promoteOutput is the stable JSON shape for `promote --json`. Discard
// variants leave TargetID, TargetType, Path nil so the JSON emits explicit
// nulls.
type promoteOutput struct {
	ID         string  `json:"id"`
	TargetID   *string `json:"target_id"`
	TargetType *string `json:"target_type"`
	Status     string  `json:"status"`
	Path       *string `json:"path"`
}

func emitPromoteOutput(cmd *cobra.Command, asJSON bool, o promoteOutput, textLine string) error {
	out := cmd.OutOrStdout()
	if asJSON {
		b, _ := json.Marshal(o)
		fmt.Fprintln(out, string(b))
		return nil
	}
	fmt.Fprintln(out, textLine)
	return nil
}

func ptrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// promotedToBareID extracts the bare `<project>.<slug>` from the inbox's
// `promoted_to` field. New writes use the wikilink form (`[[<type>.<id>]]`)
// so the index picks up the edge; legacy fixtures hold the bare ID. Accept
// either.
func promotedToBareID(v any) string {
	s, _ := v.(string)
	if strings.HasPrefix(s, "[[") && strings.HasSuffix(s, "]]") {
		s = s[2 : len(s)-2]
		if dot := strings.IndexByte(s, '.'); dot >= 0 {
			return s[dot+1:]
		}
	}
	return s
}

// formatEnumError builds a principle-4 actionable error: offending value,
// valid values, copy-pasteable corrected invocation. Pass exampleCmd="" to
// omit the corrected line (used for state-conflict errors with no valid
// retry).
func formatEnumError(field, got string, valid []string, exampleCmd string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "invalid value %q for %s", got, field)
	if len(valid) > 0 {
		fmt.Fprintf(&b, "\n  valid values: %s", strings.Join(valid, ", "))
	}
	if exampleCmd != "" {
		fmt.Fprintf(&b, "\n  corrected:    %s", exampleCmd)
	}
	return errors.New(b.String())
}

// promoteToTyped writes the target artifact, then flips the inbox row to
// status: promoted with provenance fields. Issue is the only target that
// resolves a project; the others ignore the project field.
func promoteToTyped(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string, target core.Type, asJSON bool, flagTags, flagAllowNewFacet []string, projectOverride string) error {
	status, _ := inbox.FrontMatter["status"].(string)
	switch status {
	case "promoted":
		recordedType, _ := inbox.FrontMatter["promoted_type"].(string)
		recordedTo := promotedToBareID(inbox.FrontMatter["promoted_to"])
		if recordedType == string(target) {
			tt, ti := recordedType, recordedTo
			return emitPromoteOutput(cmd, asJSON,
				promoteOutput{
					ID: inboxID, TargetID: &ti, TargetType: &tt,
					Status: "already_promoted",
					Path:   ptrIfNonEmpty(filepath.Join(v.Root, target.Dir(), recordedTo+".md")),
				},
				fmt.Sprintf("already promoted %s -> %s %s", inboxID, recordedType, recordedTo),
			)
		}
		return formatEnumError(
			"--as", string(target), []string{recordedType},
			fmt.Sprintf("anvil promote %s --as %s", inboxID, recordedType),
		)
	case "dropped":
		return fmt.Errorf("cannot promote a dropped entry %s: status is dropped, manual cleanup required", inboxID)
	}

	title, _ := inbox.FrontMatter["title"].(string)
	created := time.Now().UTC().Format("2006-01-02")
	// Spine targets require a non-empty description; reuse the inbox title as
	// the one-liner so promote stays a single-step operation.
	data := templateData{Title: title, Description: title, Created: created, Tags: flagTags}
	idInputs := core.IDInputs{Title: title}

	if target == core.TypeIssue {
		project := projectOverride
		if project == "" {
			project, _ = inbox.FrontMatter["suggested_project"].(string)
		}
		if project == "" {
			p, err := core.ResolveProject()
			if err != nil {
				if errors.Is(err, core.ErrNoProject) {
					return fmt.Errorf("set --project, set suggested_project on the inbox entry, or run from a git repo with a remote")
				}
				return fmt.Errorf("resolving project: %w", err)
			}
			project = p.Slug
		}
		data.Project = project
		idInputs.Project = project
	}

	targetID, err := core.NextID(v, target, idInputs)
	if err != nil {
		return fmt.Errorf("allocating ID: %w", err)
	}

	fm, err := renderFrontMatter(target, data)
	if err != nil {
		return fmt.Errorf("rendering %s template: %w", target, err)
	}
	if len(flagTags) > 0 {
		anyTags := make([]any, 0, len(flagTags))
		for _, s := range flagTags {
			anyTags = append(anyTags, s)
		}
		fm["tags"] = anyTags
	}

	dir := filepath.Join(v.Root, target.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	targetPath := filepath.Join(dir, targetID+".md")
	if err := schema.Validate(string(target), fm); err != nil {
		return renderSchemaErr(cmd, targetPath, err)
	}

	for _, f := range flagAllowNewFacet {
		if !facets.Has(f) {
			return formatEnumError("--allow-new-facet", f, facets.Names(), "")
		}
	}
	allowed := map[string]bool{}
	for _, f := range flagAllowNewFacet {
		allowed[f] = true
	}
	values, gErr := facets.CollectValues(v.Root)
	if gErr != nil {
		return fmt.Errorf("walking vault for facet values: %w", gErr)
	}
	tagsRaw, _ := fm["tags"].([]any)
	tagsStr := make([]string, 0, len(tagsRaw))
	for _, raw := range tagsRaw {
		if s, ok := raw.(string); ok {
			tagsStr = append(tagsStr, s)
		}
	}
	if errs := facets.Check(values, tagsStr, allowed); len(errs) > 0 {
		for _, e := range errs {
			e.Path = targetPath
		}
		printValidationErrors(cmd, errs)
		return ErrSchemaInvalid
	}
	tgtArt := &core.Artifact{Path: targetPath, FrontMatter: fm, Body: ""}
	if err := tgtArt.Save(); err != nil {
		return fmt.Errorf("saving %s: %w", target, err)
	}
	if err := indexAfterSave(v, tgtArt); err != nil {
		return fmt.Errorf("indexing target: %w", err)
	}

	inbox.FrontMatter["status"] = "promoted"
	inbox.FrontMatter["promoted_to"] = fmt.Sprintf("[[%s.%s]]", target, targetID)
	inbox.FrontMatter["promoted_type"] = string(target)
	inbox.FrontMatter["updated"] = created
	if err := schema.Validate(string(core.TypeInbox), inbox.FrontMatter); err != nil {
		return fmt.Errorf("inbox schema validation: %w", err)
	}
	if err := inbox.Save(); err != nil {
		return fmt.Errorf("saving inbox: %w", err)
	}
	if err := indexAfterSave(v, inbox); err != nil {
		return fmt.Errorf("indexing inbox: %w", err)
	}

	tt := string(target)
	ti := targetID
	return emitPromoteOutput(cmd, asJSON,
		promoteOutput{
			ID: inboxID, TargetID: &ti, TargetType: &tt,
			Status: "promoted",
			Path:   &targetPath,
		},
		fmt.Sprintf("promoted %s -> %s %s", inboxID, target, targetID),
	)
}

func discardInbox(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string, asJSON bool) error {
	status, _ := inbox.FrontMatter["status"].(string)
	switch status {
	case "dropped":
		return emitPromoteOutput(cmd, asJSON,
			promoteOutput{ID: inboxID, Status: "already_discarded"},
			fmt.Sprintf("already discarded %s", inboxID),
		)
	case "promoted":
		recordedType, _ := inbox.FrontMatter["promoted_type"].(string)
		recordedTo, _ := inbox.FrontMatter["promoted_to"].(string)
		return fmt.Errorf("cannot discard %s: already promoted to %s %s", inboxID, recordedType, recordedTo)
	}

	updated := time.Now().UTC().Format("2006-01-02")
	inbox.FrontMatter["status"] = "dropped"
	inbox.FrontMatter["updated"] = updated
	if err := schema.Validate(string(core.TypeInbox), inbox.FrontMatter); err != nil {
		return fmt.Errorf("inbox schema validation: %w", err)
	}
	if err := inbox.Save(); err != nil {
		return fmt.Errorf("saving inbox: %w", err)
	}
	if err := indexAfterSave(v, inbox); err != nil {
		return fmt.Errorf("indexing inbox: %w", err)
	}
	return emitPromoteOutput(cmd, asJSON,
		promoteOutput{ID: inboxID, Status: "discarded"},
		fmt.Sprintf("discarded %s", inboxID),
	)
}
