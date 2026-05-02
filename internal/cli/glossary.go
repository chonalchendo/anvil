package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
)

func newGlossaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "glossary",
		Short: "Manage _meta/glossary.md (tag vocabulary + definitions)",
	}
	cmd.AddCommand(
		newGlossaryShowCmd(),
		newGlossaryTagsCmd(),
		newGlossaryDefineCmd(),
		newGlossaryAddCmd(),
	)
	return cmd
}

func loadGlossary() (*core.Vault, *glossary.Glossary, error) {
	v, err := core.ResolveVault()
	if err != nil {
		return nil, nil, fmt.Errorf("resolving vault: %w", err)
	}
	g, err := glossary.Load(glossary.Path(v.Root))
	if err != nil {
		return nil, nil, err
	}
	return v, g, nil
}

func newGlossaryShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the glossary file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, g, err := loadGlossary()
			if err != nil {
				return err
			}
			cmd.Println("# Vault Glossary")
			cmd.Println("")
			cmd.Println("## Tags")
			for _, tag := range g.Tags() {
				cmd.Println("-", tag)
			}
			return nil
		},
	}
}

func newGlossaryTagsCmd() *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List tags, optionally filtered by --prefix <facet>/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, g, err := loadGlossary()
			if err != nil {
				return err
			}
			for _, tag := range g.Tags() {
				if prefix == "" || strings.HasPrefix(tag, prefix) {
					cmd.Println(tag)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "filter by facet prefix (e.g. domain/)")
	return cmd
}

func newGlossaryDefineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "define <term>",
		Short: "Print the definition for <term>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, g, err := loadGlossary()
			if err != nil {
				return err
			}
			def, ok := g.Definition(args[0])
			if !ok {
				return fmt.Errorf("term %q not in glossary", args[0])
			}
			cmd.Println(def)
			return nil
		},
	}
}

func newGlossaryAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Append entries to the glossary",
	}
	cmd.AddCommand(newGlossaryAddTagCmd())
	return cmd
}

func newGlossaryAddTagCmd() *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:   "tag <facet>/<name> --desc \"...\"",
		Short: "Append a tag under its facet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if desc == "" {
				return fmt.Errorf("--desc is required")
			}
			v, g, err := loadGlossary()
			if err != nil {
				return err
			}
			if err := g.AddTag(args[0], desc); err != nil {
				return err
			}
			if err := g.Save(glossary.Path(v.Root)); err != nil {
				return fmt.Errorf("saving glossary: %w", err)
			}
			cmd.Println("added", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "one-line description (required)")
	return cmd
}
