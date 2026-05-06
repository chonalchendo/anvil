package cli

import (
	"encoding/json"
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
	var flagStatus, flagTag, flagSince, flagUntil string
	var flagJSON, flagAll bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List inbox entries (default: 10 most recent raw)",
		Example: "  anvil inbox list\n  anvil inbox list --all --since 2026-05-01\n  anvil inbox list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			status := flagStatus
			if status == "" && !flagAll {
				status = "raw"
			}
			return runList(cmd, v, core.TypeInbox, listFilters{
				Status: status, Tag: flagTag, Since: flagSince, Until: flagUntil,
			}, flagJSON, flagLimit)
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "filter by status (exact match)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag (substring match)")
	cmd.Flags().StringVar(&flagSince, "since", "", "include only entries created on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&flagUntil, "until", "", "include only entries created on or before YYYY-MM-DD")
	cmd.Flags().IntVar(&flagLimit, "limit", defaultListLimit, "maximum results to return (default 10)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "include promoted and dropped entries (default: only raw)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	return cmd
}

func newInboxShowCmd() *cobra.Command {
	var flagJSON, flagFull bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display an inbox entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			return runShow(cmd, v, core.TypeInbox, args[0], flagJSON, flagFull)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.Flags().BoolVar(&flagFull, "full", false, "include body (capped at 500 lines)")
	return cmd
}

func newInboxPromoteCmd() *cobra.Command {
	c := newPromoteCmd()
	c.Use = "promote <id>"
	return c
}
