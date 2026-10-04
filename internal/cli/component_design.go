package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
)

// componentDesignKindFacet is the glossary facet that holds the component-design kind
// vocabulary. Kinds round-trip through the same registry as tags so the
// existing parse/save/dedup machinery is reused (see internal/glossary).
const componentDesignKindFacet = "kind"

func newComponentDesignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "component-design",
		Short:        "Manage component designs and their registered kinds",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newComponentDesignKindsCmd())
	return cmd
}

func newComponentDesignKindsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kinds",
		Short:        "Register and list component design kinds",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newComponentDesignKindsAddCmd(), newComponentDesignKindsListCmd())
	return cmd
}

func newComponentDesignKindsAddCmd() *cobra.Command {
	var (
		flagDesc   string
		flagUpdate bool
	)
	cmd := &cobra.Command{
		Use:   "add <name> [--desc \"...\"]",
		Short: "Register a component design kind in the vault glossary (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if strings.ContainsRune(name, '/') {
				return fmt.Errorf("kind %q must be a bare name, not a facet path", name)
			}
			tag := componentDesignKindFacet + "/" + name
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			path := glossary.Path(v.Root)
			g, err := glossary.Load(path)
			if err != nil {
				return err
			}
			existing, hadIt := g.FindTagDesc(tag)
			if hadIt && existing == flagDesc {
				fmt.Fprintln(cmd.OutOrStdout(), path)
				return nil
			}
			if hadIt && !flagUpdate {
				return fmt.Errorf("kind %q already registered with a different description\n  existing: %s\n  new:      %s\n  corrected: anvil component-design kinds add %s --desc %q --update",
					name, existing, flagDesc, name, flagDesc)
			}
			if hadIt && flagUpdate {
				_ = g.UpdateTagDesc(tag, flagDesc)
			} else if err := g.AddTag(tag, flagDesc); err != nil {
				return err
			}
			if err := g.Save(path); err != nil {
				return fmt.Errorf("saving glossary: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDesc, "desc", "", "one-line description of the kind")
	cmd.Flags().BoolVar(&flagUpdate, "update", false, "rewrite an existing kind's description")
	return cmd
}

func newComponentDesignKindsListCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered component design kinds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			g, err := glossary.Load(glossary.Path(v.Root))
			if err != nil {
				return err
			}
			kinds := registeredKinds(g)
			out := cmd.OutOrStdout()
			if flagJSON {
				b, err := json.Marshal(kinds)
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			for _, k := range kinds {
				fmt.Fprintln(out, k)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON array of kind names")
	return cmd
}

// registeredKinds returns the sorted bare kind names recorded in the glossary
// `kind/` facet.
func registeredKinds(g *glossary.Glossary) []string {
	var out []string
	prefix := componentDesignKindFacet + "/"
	for _, tag := range g.Tags() {
		if name, ok := strings.CutPrefix(tag, prefix); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// checkComponentDesignKind validates that fm["kind"] is a registered component design kind.
// Returns nil when the kind is registered (or absent — the schema's required
// check owns the empty case), else a ValidationError naming the registered set.
func checkComponentDesignKind(vaultRoot, path string, fm map[string]any) *errfmt.ValidationError {
	kind, _ := fm["kind"].(string)
	if kind == "" {
		return nil
	}
	g, err := glossary.Load(glossary.Path(vaultRoot))
	if err == nil && g.HasTag(componentDesignKindFacet+"/"+kind) {
		return nil
	}
	registered := []string{}
	if g != nil {
		registered = registeredKinds(g)
	}
	e := errfmt.NewValidationError(errfmt.CodeUnknownFacetValue, path, "kind", kind).
		WithExpected(registered)
	if sug, ok := facets.Suggest(kind, registered); ok {
		e.WithSuggest(sug).WithFix(fmt.Sprintf(
			"use --kind %s, or register %q first: anvil component-design kinds add %s", sug, kind, kind))
	} else {
		e.WithFix(fmt.Sprintf(
			"register it first: anvil component-design kinds add %s (then `anvil component-design kinds list` shows all kinds)", kind))
	}
	return e
}
