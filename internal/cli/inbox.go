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
	var flagTitle, flagSuggestedType, flagSuggestedProject string
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

			dir := filepath.Join(v.Root, core.TypeInbox.Dir())
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}
			path := filepath.Join(dir, id+".md")
			a := &core.Artifact{Path: path, FrontMatter: fm, Body: ""}
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
				if err := os.Remove(inboxPath); err != nil {
					return fmt.Errorf("deleting inbox entry: %w", err)
				}
				cmd.Println("discarded", id)
				return nil

			case "learning":
				return promoteToLearning(cmd, v, a, id)

			case "design":
				return fmt.Errorf("promote to design is out of scope in v0.1")

			case "issue":
				return promoteToIssue(cmd, v, a, id)

			case "thread":
				return promoteToThread(cmd, v, a, id)

			default:
				return fmt.Errorf("unknown type %q (issue|thread|design|learning|discard)", target)
			}
		},
	}

	cmd.Flags().StringVar(&flagAs, "as", "", "promotion target type (overrides suggested_type)")
	return cmd
}

func promoteToIssue(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string) error {
	// Determine project: suggested_project wins; fall back to auto-detected.
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

	title, _ := inbox.FrontMatter["title"].(string)

	issueID, err := core.NextID(v, core.TypeIssue, core.IDInputs{Title: title, Project: project})
	if err != nil {
		return fmt.Errorf("allocating ID: %w", err)
	}

	created := time.Now().UTC().Format("2006-01-02")
	data := templateData{
		Title:   title,
		Created: created,
		Project: project,
	}

	fm, err := renderFrontMatter(core.TypeIssue, data)
	if err != nil {
		return fmt.Errorf("rendering issue template: %w", err)
	}

	if err := schema.Validate(string(core.TypeIssue), fm); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}

	dir := filepath.Join(v.Root, core.TypeIssue.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	issuePath := filepath.Join(dir, issueID+".md")
	art := &core.Artifact{Path: issuePath, FrontMatter: fm, Body: ""}
	if err := art.Save(); err != nil {
		return fmt.Errorf("saving issue: %w", err)
	}

	// Remove inbox file only after issue is written successfully.
	inboxPath := filepath.Join(v.Root, core.TypeInbox.Dir(), inboxID+".md")
	if err := os.Remove(inboxPath); err != nil {
		return fmt.Errorf("deleting inbox entry: %w", err)
	}

	cmd.Println("issue", issueID)
	return nil
}

func promoteToThread(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string) error {
	title, _ := inbox.FrontMatter["title"].(string)

	threadID, err := core.NextID(v, core.TypeThread, core.IDInputs{Title: title})
	if err != nil {
		return fmt.Errorf("allocating ID: %w", err)
	}

	created := time.Now().UTC().Format("2006-01-02")
	data := templateData{
		Title:   title,
		Created: created,
	}

	fm, err := renderFrontMatter(core.TypeThread, data)
	if err != nil {
		return fmt.Errorf("rendering thread template: %w", err)
	}

	if err := schema.Validate(string(core.TypeThread), fm); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}

	dir := filepath.Join(v.Root, core.TypeThread.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	threadPath := filepath.Join(dir, threadID+".md")
	art := &core.Artifact{Path: threadPath, FrontMatter: fm, Body: ""}
	if err := art.Save(); err != nil {
		return fmt.Errorf("saving thread: %w", err)
	}

	inboxPath := filepath.Join(v.Root, core.TypeInbox.Dir(), inboxID+".md")
	if err := os.Remove(inboxPath); err != nil {
		return fmt.Errorf("deleting inbox entry: %w", err)
	}

	cmd.Println("thread", threadID)
	return nil
}

func promoteToLearning(cmd *cobra.Command, v *core.Vault, inbox *core.Artifact, inboxID string) error {
	title, _ := inbox.FrontMatter["title"].(string)

	learningID, err := core.NextID(v, core.TypeLearning, core.IDInputs{Title: title})
	if err != nil {
		return fmt.Errorf("allocating ID: %w", err)
	}

	created := time.Now().UTC().Format("2006-01-02")
	data := templateData{Title: title, Created: created}

	fm, err := renderFrontMatter(core.TypeLearning, data)
	if err != nil {
		return fmt.Errorf("rendering learning template: %w", err)
	}

	if err := schema.Validate(string(core.TypeLearning), fm); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}

	dir := filepath.Join(v.Root, core.TypeLearning.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	learningPath := filepath.Join(dir, learningID+".md")
	art := &core.Artifact{Path: learningPath, FrontMatter: fm, Body: ""}
	if err := art.Save(); err != nil {
		return fmt.Errorf("saving learning: %w", err)
	}

	inboxPath := filepath.Join(v.Root, core.TypeInbox.Dir(), inboxID+".md")
	if err := os.Remove(inboxPath); err != nil {
		return fmt.Errorf("deleting inbox entry: %w", err)
	}

	cmd.Println("learning", learningID)
	return nil
}
