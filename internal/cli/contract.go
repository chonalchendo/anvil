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

// contractKindFacet is the glossary facet that holds the contract-kind
// vocabulary. Kinds round-trip through the same registry as tags so the
// existing parse/save/dedup machinery is reused (see internal/glossary).
const contractKindFacet = "kind"

func newContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "contract",
		Short:        "Manage contracts and their registered kinds",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newContractKindsCmd())
	return cmd
}

func newContractKindsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kinds",
		Short:        "Register and list contract kinds",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newContractKindsAddCmd(), newContractKindsListCmd())
	return cmd
}

func newContractKindsAddCmd() *cobra.Command {
	var (
		flagDesc   string
		flagUpdate bool
	)
	cmd := &cobra.Command{
		Use:   "add <name> [--desc \"...\"]",
		Short: "Register a contract kind in the vault glossary (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if strings.ContainsRune(name, '/') {
				return fmt.Errorf("kind %q must be a bare name, not a facet path", name)
			}
			tag := contractKindFacet + "/" + name
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
				return fmt.Errorf("kind %q already registered with a different description\n  existing: %s\n  new:      %s\n  corrected: anvil contract kinds add %s --desc %q --update",
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

func newContractKindsListCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered contract kinds",
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
	prefix := contractKindFacet + "/"
	for _, tag := range g.Tags() {
		if name, ok := strings.CutPrefix(tag, prefix); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// checkContractKind validates that fm["kind"] is a registered contract kind.
// Returns nil when the kind is registered (or absent — the schema's required
// check owns the empty case), else a ValidationError naming the registered set.
func checkContractKind(vaultRoot, path string, fm map[string]any) *errfmt.ValidationError {
	kind, _ := fm["kind"].(string)
	if kind == "" {
		return nil
	}
	g, err := glossary.Load(glossary.Path(vaultRoot))
	if err == nil && g.HasTag(contractKindFacet+"/"+kind) {
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
			"use --kind %s, or register %q first: anvil contract kinds add %s", sug, kind, kind))
	} else {
		e.WithFix(fmt.Sprintf(
			"register it first: anvil contract kinds add %s (then `anvil contract kinds list` shows all kinds)", kind))
	}
	return e
}
