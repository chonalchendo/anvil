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
		flagSeverity      string
		flagMilestone     string
		flagAcceptance    []string
		flagBody          string
		flagBodyFile      string
		flagGoal          string
		flagDescription   string
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
				return promoteToTyped(cmd, v, a, id, core.Type(flagAs), flagJSON, flagTags, flagAllowNewFacet, flagProjectLocal, flagSeverity, flagMilestone, flagAcceptance, flagBody, flagBodyFile, flagGoal, flagDescription)
			}
		},
	}

	cmd.Flags().StringVar(&flagAs, "as", "", "promotion target type (issue|thread|learning|discard)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "tags to seed on promoted artifact")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet(s) to suppress novelty gate for")
	cmd.Flags().StringVar(&flagProjectLocal, "project", "", "project slug for the promoted issue (overrides inbox suggested_project and resolver)")
	cmd.Flags().StringVar(&flagSeverity, "severity", "", "issue severity (low|medium|high|critical; issue only)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "milestone slug or wikilink to assign (issue only)")
	cmd.Flags().StringArrayVar(&flagAcceptance, "acceptance", nil, "acceptance criterion to add (repeatable; issue only)")
	cmd.Flags().StringVar(&flagBody, "body", "", "body content for the promoted artifact (literal, or '-' to read stdin; issue only)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read body from <path> (issue only; mutually exclusive with --body)")
	cmd.Flags().StringVar(&flagGoal, "goal", "", "issue goal — terminal predicate (issue only; defaults to the inbox title)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "one-line description (defaults to the inbox title)")
	_ = cmd.MarkFlagRequired("as")
	return cmd
}

// promoteOutput is the stable JSON shape for `promote --json`. For promoted
// targets, ID holds the new artifact's id and SourceID holds the inbox entry
// that was consumed; this lets callers pipe `.id` into subsequent commands
// without extracting `.target_id`. Discard variants leave TargetID, TargetType,
// Path, and SourceID nil so the JSON emits explicit nulls for those fields.
type promoteOutput struct {
	ID         string  `json:"id"`
	SourceID   *string `json:"source_id,omitempty"`
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
func promoteToTyped(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string, target core.Type, asJSON bool, flagTags, flagAllowNewFacet []string, projectOverride, flagSeverity, flagMilestone string, flagAcceptance []string, flagBody, flagBodyFile, flagGoal, flagDescription string) error {
	status, _ := inbox.FrontMatter["status"].(string)
	switch status {
	case "promoted":
		recordedType, _ := inbox.FrontMatter["promoted_type"].(string)
		recordedTo := promotedToBareID(inbox.FrontMatter["promoted_to"])
		if recordedType == string(target) {
			tt, ti, si := recordedType, recordedTo, inboxID
			return emitPromoteOutput(cmd, asJSON,
				promoteOutput{
					ID: recordedTo, SourceID: &si, TargetID: &ti, TargetType: &tt,
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
	// Spine targets require a non-empty description, and issues a non-empty
	// goal; default both to the inbox title so promote stays single-step, but
	// let --description / --goal override — a long inbox title overflows the
	// 120-char schema cap, and a promoted issue should reach create-issue
	// parity. Goal is ignored by non-issue templates.
	description := title
	if flagDescription != "" {
		description = flagDescription
	}
	goal := title
	if flagGoal != "" {
		goal = flagGoal
	}
	data := templateData{Title: title, Description: description, Goal: goal, Created: created, Tags: flagTags}

	var (
		targetID   string
		targetPath string
	)
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
		// Mint the numbered <project>.NNNN.<slug> id via the same allocator
		// create uses, so promoted and created issues share one id scheme.
		var err error
		if targetID, targetPath, err = core.AllocateIssueID(v, project, title, ""); err != nil {
			return fmt.Errorf("allocating ID: %w", err)
		}
	} else {
		var err error
		if targetID, err = core.NextID(v, target, core.IDInputs{Title: title}); err != nil {
			return fmt.Errorf("allocating ID: %w", err)
		}
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
	if flagSeverity != "" && target == core.TypeIssue {
		fm["severity"] = flagSeverity
	}
	if flagMilestone != "" && target == core.TypeIssue {
		fm["milestone"] = normalizeMilestone(flagMilestone)
	}
	if len(flagAcceptance) > 0 && target == core.TypeIssue {
		anyAcc := make([]any, len(flagAcceptance))
		for i, s := range flagAcceptance {
			anyAcc[i] = s
		}
		fm["acceptance"] = anyAcc
	}

	// Determine body for the promoted artifact. Issue targets need the required
	// heading scaffold when no explicit body is supplied; other types reuse the
	// inbox body as-is.
	body := inbox.Body
	userAuthoredBody := false
	if target == core.TypeIssue {
		if flagBody != "" || flagBodyFile != "" {
			b, err := readBody(cmd, flagBody, flagBodyFile)
			if err != nil {
				return err
			}
			body = b
			userAuthoredBody = true
		} else {
			// Scaffold the required issue sections so the promoted artifact
			// passes `anvil validate` without a follow-up edit round-trip.
			body = core.ScaffoldSections(core.RequiredIssueSections)
		}
	}

	dir := filepath.Join(v.Root, target.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if targetPath == "" {
		targetPath = filepath.Join(dir, targetID+".md")
	}

	// AllocateIssueID resolves a colliding slug to the existing issue (create
	// uses that for its drift check); promote has no drift path, so refuse
	// rather than overwrite a live issue.
	if _, statErr := os.Stat(targetPath); statErr == nil {
		return fmt.Errorf("issue %s already exists (slug collision with this inbox title); rename the inbox before promoting", targetID)
	}

	// Route through the same validator create uses so the two paths accept the
	// identical artifact set: schema + facet novelty, plus — for an authored
	// body — required headings AND wikilink resolution. Inline-validating here
	// (the prior shape) let an unresolved [[wikilink]] through promote that
	// create rejects.
	if err := validateBeforeCreate(cmd, v, target, targetPath, fm, body, userAuthoredBody, flagAllowNewFacet, asJSON); err != nil {
		return err
	}

	tgtArt := &core.Artifact{Path: targetPath, FrontMatter: fm, Body: body}
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
	si := inboxID
	return emitPromoteOutput(cmd, asJSON,
		promoteOutput{
			ID: targetID, SourceID: &si, TargetID: &ti, TargetType: &tt,
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
