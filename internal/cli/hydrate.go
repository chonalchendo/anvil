package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
)

// newHydrateCmd assembles an issue's methodology-spine context closure into one
// bundle of linked bodies: the issue, its milestone, the milestone's designs,
// the issue's contracts→conventions, and its prior learnings. A spine edge whose
// target does not resolve on disk makes the command exit non-zero naming it,
// rather than silently omitting it.
func newHydrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hydrate <issue>",
		Short:   "Assemble an issue's linked-context closure (issue → milestone → designs, contracts → conventions, learnings) as bodies; a dangling spine edge exits non-zero naming it",
		Args:    namedArgs("anvil hydrate <issue>", []string{"<issue>"}, 1, 1),
		Example: "  anvil hydrate anvil.0148.assemble-the-linked",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			return runHydrate(cmd, v, canonicalArtifactID(v, core.TypeIssue, args[0]))
		},
	}
	return cmd
}

// spineNode is one resolved artifact in the assembled closure: its type, canonical
// id, frontmatter status (so a non-active design reads as advisory), and body.
type spineNode struct {
	Type   core.Type
	ID     string
	Status string
	Body   string
	Path   string
}

// brokenEdge is a declared spine wikilink whose target does not resolve on disk.
// Target carries the full type-qualified wikilink (e.g. milestone.foo.ghost), so
// the edge type needs no separate field.
type brokenEdge struct {
	Source string // "<type> <id>" of the artifact declaring the edge
	Target string // the type-qualified wikilink target that failed to resolve
}

// hydration accumulates the assembled closure and any broken edges as the walk
// descends the fixed methodology spine.
type hydration struct {
	nodes  []spineNode
	broken []brokenEdge
}

// walk resolves target of linkType declared by sourceDesc via forward file
// resolution (target file exists?), never incoming-edge presence — a
// prefix-retaining design/convention link resolves forward but registers no
// incoming edge, so an incoming-edge check would false-flag it. A missing target
// records a broken edge and returns nil so the walk continues; the loaded
// artifact is returned so the caller can descend into its own links.
func (h *hydration) walk(v *core.Vault, sourceDesc string, linkType core.Type, target string) (*core.Artifact, error) {
	id := canonicalArtifactID(v, linkType, target)
	a, err := core.LoadArtifact(resolveArtifactPath(v.Root, linkType, id))
	if err != nil {
		if os.IsNotExist(err) {
			h.broken = append(h.broken, brokenEdge{Source: sourceDesc, Target: target})
			return nil, nil
		}
		return nil, fmt.Errorf("loading %s %s: %w", linkType, target, err)
	}
	h.nodes = append(h.nodes, nodeOf(linkType, id, a))
	return a, nil
}

func nodeOf(t core.Type, id string, a *core.Artifact) spineNode {
	status, _ := a.FrontMatter["status"].(string)
	return spineNode{Type: t, ID: id, Status: status, Body: strings.TrimPrefix(a.Body, "\n"), Path: a.Path}
}

func runHydrate(cmd *cobra.Command, v *core.Vault, issueID string) error {
	iss, err := core.LoadArtifact(resolveArtifactPath(v.Root, core.TypeIssue, issueID))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrArtifactNotFound, issueID)
		}
		return fmt.Errorf("loading issue: %w", err)
	}

	h := &hydration{nodes: []spineNode{nodeOf(core.TypeIssue, issueID, iss)}}
	issueSrc := "issue " + issueID

	// issue → milestone → {product-design, system-design}
	for _, mt := range linkTargetsOfType(iss, core.TypeMilestone) {
		ms, err := h.walk(v, issueSrc, core.TypeMilestone, mt)
		if err != nil {
			return err
		}
		if ms == nil {
			continue
		}
		msSrc := "milestone " + mt
		for _, dtype := range []core.Type{core.TypeProductDesign, core.TypeSystemDesign} {
			for _, dt := range linkTargetsOfType(ms, dtype) {
				if _, err := h.walk(v, msSrc, dtype, dt); err != nil {
					return err
				}
			}
		}
	}

	// issue → contract → convention
	for _, ct := range linkTargetsOfType(iss, core.TypeContract) {
		c, err := h.walk(v, issueSrc, core.TypeContract, ct)
		if err != nil {
			return err
		}
		if c == nil {
			continue
		}
		for _, cv := range linkTargetsOfType(c, core.TypeConvention) {
			if _, err := h.walk(v, "contract "+ct, core.TypeConvention, cv); err != nil {
				return err
			}
		}
	}

	// issue → prior learnings
	for _, lt := range linkTargetsOfType(iss, core.TypeLearning) {
		if _, err := h.walk(v, issueSrc, core.TypeLearning, lt); err != nil {
			return err
		}
	}

	emitHydration(cmd, h.nodes)
	if len(h.broken) > 0 {
		return brokenSpineError(h.broken)
	}
	return nil
}

// emitHydration prints each node's body under a `=== <type> <id> (status) ===`
// header to stdout, capping each body at showBodyLineCap. Prose (clip hints, the
// node count) goes to stderr so a large fan-out doesn't pollute the bundle.
func emitHydration(cmd *cobra.Command, nodes []spineNode) {
	w := cmd.OutOrStdout()
	for _, n := range nodes {
		status := n.Status
		if status == "" {
			status = "unset"
		}
		fmt.Fprintf(w, "=== %s %s (status: %s) ===\n", n.Type, n.ID, status)
		body := n.Body
		if lines := strings.Split(body, "\n"); body != "" && len(lines) > showBodyLineCap {
			body = strings.Join(lines[:showBodyLineCap], "\n")
			cmd.PrintErrln(output.BodyClipHint(showBodyLineCap, len(lines), n.Path))
		}
		fmt.Fprintln(w, body)
	}
	cmd.PrintErrf("hydrated %d spine node(s)\n", len(nodes))
}

// brokenSpineError names every dangling spine edge so the failure is actionable
// (which artifact declares which unresolvable target), not just a non-zero exit.
func brokenSpineError(broken []brokenEdge) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%d broken spine edge(s):", len(broken))
	for _, e := range broken {
		fmt.Fprintf(&b, "\n  %s → [[%s]] (target not found)", e.Source, e.Target)
	}
	return errors.New(b.String())
}
