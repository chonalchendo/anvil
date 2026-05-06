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
	var flagAs string
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promote an inbox entry to a typed artifact",
		Long:  "Operates on inbox entries (the only promotable type today). --as selects the target type.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			validAs := []string{"issue", "thread", "design", "learning", "discard"}
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
				return discardInbox(cmd, a, id, flagJSON)
			case "design":
				return fmt.Errorf("promote to design is out of scope in v0.1")
			default:
				return promoteToTyped(cmd, v, a, id, core.Type(flagAs), flagJSON)
			}
		},
	}

	cmd.Flags().StringVar(&flagAs, "as", "", "promotion target type (issue|thread|design|learning|discard)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
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
	if asJSON {
		b, _ := json.Marshal(o)
		cmd.Println(string(b))
		return nil
	}
	cmd.Println(textLine)
	return nil
}

func ptrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
func promoteToTyped(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string, target core.Type, asJSON bool) error {
	status, _ := inbox.FrontMatter["status"].(string)
	switch status {
	case "promoted":
		recordedType, _ := inbox.FrontMatter["promoted_type"].(string)
		recordedTo, _ := inbox.FrontMatter["promoted_to"].(string)
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
	data := templateData{Title: title, Description: title, Created: created}
	idInputs := core.IDInputs{Title: title}

	if target == core.TypeIssue {
		project, _ := inbox.FrontMatter["suggested_project"].(string)
		if project == "" {
			p, err := core.ResolveProject()
			if err != nil {
				if errors.Is(err, core.ErrNoProject) {
					return fmt.Errorf("set suggested_project or run from a git repo with a remote")
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
	if err := schema.Validate(string(target), fm); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}

	dir := filepath.Join(v.Root, target.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	targetPath := filepath.Join(dir, targetID+".md")
	if err := (&core.Artifact{Path: targetPath, FrontMatter: fm, Body: ""}).Save(); err != nil {
		return fmt.Errorf("saving %s: %w", target, err)
	}

	inbox.FrontMatter["status"] = "promoted"
	inbox.FrontMatter["promoted_to"] = targetID
	inbox.FrontMatter["promoted_type"] = string(target)
	inbox.FrontMatter["updated"] = created
	if err := schema.Validate(string(core.TypeInbox), inbox.FrontMatter); err != nil {
		return fmt.Errorf("inbox schema validation: %w", err)
	}
	if err := inbox.Save(); err != nil {
		return fmt.Errorf("saving inbox: %w", err)
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

func discardInbox(cmd *cobra.Command, inbox *core.Artifact, inboxID string, asJSON bool) error {
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
	return emitPromoteOutput(cmd, asJSON,
		promoteOutput{ID: inboxID, Status: "discarded"},
		fmt.Sprintf("discarded %s", inboxID),
	)
}
