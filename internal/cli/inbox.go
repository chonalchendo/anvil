package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func newInboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Manage inbox artifacts",
	}
	cmd.AddCommand(
		newInboxAddCmd(),
		newInboxListCmd(),
		newInboxShowCmd(),
		newInboxPromoteCmd(),
	)
	return cmd
}

func newInboxAddCmd() *cobra.Command {
	var flagTitle, flagSuggestedType, flagSuggestedProject, flagBody string
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create an inbox entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			id, err := core.NextID(v, core.TypeInbox, core.IDInputs{Title: flagTitle})
			if err != nil {
				return fmt.Errorf("allocating ID: %w", err)
			}

			created := time.Now().UTC().Format("2006-01-02")
			data := templateData{
				Title:            flagTitle,
				Created:          created,
				SuggestedType:    flagSuggestedType,
				SuggestedProject: flagSuggestedProject,
			}

			fm, err := renderFrontMatter(core.TypeInbox, data)
			if err != nil {
				return fmt.Errorf("rendering template: %w", err)
			}
			if err := schema.Validate(string(core.TypeInbox), fm); err != nil {
				return fmt.Errorf("schema validation: %w", err)
			}

			body, err := readBody(cmd, flagBody)
			if err != nil {
				return err
			}

			dir := filepath.Join(v.Root, core.TypeInbox.Dir())
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}
			path := filepath.Join(dir, id+".md")
			a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}

			if flagJSON {
				out, _ := json.Marshal(map[string]string{"id": id, "path": path})
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			} else {
				cmd.Println(path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "inbox entry title (required)")
	cmd.Flags().StringVar(&flagSuggestedType, "suggested-type", "", "suggested artifact type (issue|design|learning|discard)")
	cmd.Flags().StringVar(&flagSuggestedProject, "suggested-project", "", "suggested project slug")
	cmd.Flags().StringVar(&flagBody, "body", "", "inbox body content (or pipe via stdin)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newInboxListCmd() *cobra.Command {
	var flagStatus, flagTag string
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List inbox entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			return runList(cmd, v, core.TypeInbox, listFilters{Status: flagStatus, Tag: flagTag}, flagJSON)
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "filter by status (exact match)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag (substring match)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	return cmd
}

func newInboxShowCmd() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display an inbox entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			return runShow(cmd, v, core.TypeInbox, args[0], flagJSON)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	return cmd
}

func newInboxPromoteCmd() *cobra.Command {
	var flagAs string

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promote an inbox entry to a typed artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

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

			target := flagAs
			if target == "" {
				target, _ = a.FrontMatter["suggested_type"].(string)
			}
			if target == "" {
				return fmt.Errorf("set suggested_type or pass --as <type> before promoting (issue|thread|design|learning|discard)")
			}

			switch target {
			case "discard":
				return discardInbox(cmd, a, id)
			case "design":
				return fmt.Errorf("promote to design is out of scope in v0.1")
			case "issue", "thread", "learning":
				return promoteToTyped(cmd, v, a, id, core.Type(target))
			default:
				return fmt.Errorf("unknown type %q (issue|thread|design|learning|discard)", target)
			}
		},
	}

	cmd.Flags().StringVar(&flagAs, "as", "", "promotion target type (issue|thread|design|learning|discard)")
	return cmd
}

// promoteToTyped writes the target artifact, then flips the inbox row to
// status: promoted with provenance fields. Issue is the only target that
// resolves a project; the others ignore the project field.
func promoteToTyped(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string, target core.Type) error {
	title, _ := inbox.FrontMatter["title"].(string)
	created := time.Now().UTC().Format("2006-01-02")
	data := templateData{Title: title, Created: created}
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

	cmd.Println("promoted", inboxID, "->", string(target), targetID)
	return nil
}

func discardInbox(cmd *cobra.Command, inbox *core.Artifact, inboxID string) error {
	updated := time.Now().UTC().Format("2006-01-02")
	inbox.FrontMatter["status"] = "dropped"
	inbox.FrontMatter["updated"] = updated
	if err := schema.Validate(string(core.TypeInbox), inbox.FrontMatter); err != nil {
		return fmt.Errorf("inbox schema validation: %w", err)
	}
	if err := inbox.Save(); err != nil {
		return fmt.Errorf("saving inbox: %w", err)
	}
	cmd.Println("discarded", inboxID)
	return nil
}
