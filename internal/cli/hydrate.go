package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// newHydrateCmd assembles an issue's methodology-spine context closure into one
// bundle of linked bodies: the issue, its milestone, the milestone's designs, the
// conventions those designs and the issue's contracts govern by, and its prior
// learnings. A spine edge whose target does not resolve on disk makes the command
// exit non-zero naming it, rather than silently omitting it.
func newHydrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hydrate <issue>",
		Short:   "Assemble an issue's linked-context closure (issue → milestone → designs → conventions, contracts → conventions, learnings) as bodies; a dangling spine edge exits non-zero naming it",
		Args:    namedArgs("anvil hydrate <issue>", []string{"<issue>"}, 1, 1),
		Example: "  anvil hydrate anvil.0148.assemble-the-linked",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			tldr, err := cmd.Flags().GetBool("tldr")
			if err != nil {
				return err
			}
			// Node ids are canonical, never the on-disk basename — hydrate must
			// print the same id shape every other read verb emits.
			id, _ := core.ResolveArtifact(v, core.TypeIssue, args[0])
			return runHydrate(cmd, v, id, tldr)
		},
	}
	cmd.Flags().Bool("tldr", false, "emit each spine node's frontmatter + ## TL;DR only instead of full bodies — a compact boundary map to scan before drilling into a node with `anvil show <type> <id> --body`")
	return cmd
}

// spineNode is one resolved artifact in the assembled closure: its type, canonical
// id, frontmatter status (so a non-active design reads as advisory), body, and the
// parsed frontmatter (the --tldr digest renders it in place of the body).
type spineNode struct {
	Type        core.Type
	ID          string
	Status      string
	Body        string
	Path        string
	FrontMatter map[string]any
}

// brokenEdge is a declared spine wikilink whose target does not resolve on disk.
// Target carries the full type-qualified wikilink (e.g. milestone.foo.ghost), so
// the edge type needs no separate field.
type brokenEdge struct {
	Source string // "<type> <id>" of the artifact declaring the edge
	Target string // the type-qualified wikilink target that failed to resolve
}

// hydration accumulates the assembled closure and any broken edges as the walk
// descends the fixed methodology spine. seen keys the nodes already emitted, so a
// convention reachable by two rails (a contract and a design) enters the bundle
// once — a duplicated body is pure context cost to the reader.
type hydration struct {
	nodes  []spineNode
	broken []brokenEdge
	seen   map[string]bool
}

// walk resolves target of linkType declared by sourceDesc via forward file
// resolution (target file exists?), never incoming-edge presence — forward
// resolution keeps hydrate independent of index freshness, so a vault whose
// links table predates the canonical-target fix still walks correctly. A
// missing target records a broken edge and returns nil so the walk continues;
// the loaded artifact is returned so the caller can descend into its own links.
func (h *hydration) walk(v *core.Vault, sourceDesc string, linkType core.Type, target string) (*core.Artifact, error) {
	basename := canonicalArtifactID(v, linkType, target)
	a, err := core.LoadArtifact(resolveArtifactPath(v.Root, linkType, basename))
	if err != nil {
		if os.IsNotExist(err) {
			h.broken = append(h.broken, brokenEdge{Source: sourceDesc, Target: target})
			return nil, nil
		}
		return nil, fmt.Errorf("loading %s %s: %w", linkType, target, err)
	}
	// The basename loads the file; the node reports the canonical id — a bare
	// back-catalogue filename must not leak into hydrate's output.
	id := core.CanonicalID(linkType, basename)
	if key := string(linkType) + " " + id; !h.seen[key] {
		h.seen[key] = true
		h.nodes = append(h.nodes, nodeOf(linkType, id, a))
	}
	return a, nil
}

func nodeOf(t core.Type, id string, a *core.Artifact) spineNode {
	status, _ := a.FrontMatter["status"].(string)
	return spineNode{Type: t, ID: id, Status: status, Body: strings.TrimPrefix(a.Body, "\n"), Path: a.Path, FrontMatter: a.FrontMatter}
}

func runHydrate(cmd *cobra.Command, v *core.Vault, issueID string, tldr bool) error {
	h, err := assembleHydration(v, issueID)
	if err != nil {
		return err
	}
	emitHydration(cmd, h.nodes, tldr)
	if len(h.broken) > 0 {
		return brokenSpineError(h.broken)
	}
	return nil
}

// assembleHydration walks the methodology spine from issueID and returns the
// accumulated closure (resolved nodes + any broken edges). One walk feeds both
// consumers: the `hydrate` command emits it, the `build` driver folds it into a
// dispatch task body — so an interactive and a headless implementer open the
// same box.
func assembleHydration(v *core.Vault, issueID string) (*hydration, error) {
	// Callers hand a canonical id (walkability derives one per file), which may
	// differ from the on-disk basename until the back catalogue is renamed.
	_, issPath := core.ResolveArtifact(v, core.TypeIssue, issueID)
	iss, err := core.LoadArtifact(issPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrArtifactNotFound, issueID)
		}
		return nil, fmt.Errorf("loading issue: %w", err)
	}

	h := &hydration{
		nodes: []spineNode{nodeOf(core.TypeIssue, issueID, iss)},
		seen:  map[string]bool{string(core.TypeIssue) + " " + issueID: true},
	}
	issueSrc := "issue " + issueID

	// issue → milestone → {product-design, system-design} → convention
	for _, mt := range linkTargetsOfType(iss, core.TypeMilestone) {
		ms, err := h.walk(v, issueSrc, core.TypeMilestone, mt)
		if err != nil {
			return nil, err
		}
		if ms == nil {
			continue
		}
		msSrc := "milestone " + mt
		for _, dtype := range []core.Type{core.TypeProductDesign, core.TypeSystemDesign} {
			for _, dt := range linkTargetsOfType(ms, dtype) {
				d, err := h.walk(v, msSrc, dtype, dt)
				if err != nil {
					return nil, err
				}
				if d == nil {
					continue
				}
				if err := h.descendConventions(v, string(dtype)+" "+dt, d); err != nil {
					return nil, err
				}
			}
		}
	}

	// issue → contract → convention
	for _, ct := range linkTargetsOfType(iss, core.TypeContract) {
		c, err := h.walk(v, issueSrc, core.TypeContract, ct)
		if err != nil {
			return nil, err
		}
		if c == nil {
			continue
		}
		if err := h.descendConventions(v, "contract "+ct, c); err != nil {
			return nil, err
		}
	}

	// issue → prior learnings
	for _, lt := range linkTargetsOfType(iss, core.TypeLearning) {
		if _, err := h.walk(v, issueSrc, core.TypeLearning, lt); err != nil {
			return nil, err
		}
	}

	return h, nil
}

// descendConventions walks an artifact's convention links — the shared last hop of
// both governing rails. A design carries the house style every issue under it must
// obey, so reaching conventions only through a contract left them unreachable for
// any issue whose repo declares no contract.
func (h *hydration) descendConventions(v *core.Vault, sourceDesc string, a *core.Artifact) error {
	for _, cv := range linkTargetsOfType(a, core.TypeConvention) {
		if _, err := h.walk(v, sourceDesc, core.TypeConvention, cv); err != nil {
			return err
		}
	}
	return nil
}

// emitHydration prints the manifest, then each node under a `=== <type> <id>
// (status) ===` header, to stdout. Full mode prints the body capped at
// showBodyLineCap; --tldr prints the compact digest (frontmatter + any `## TL;DR`)
// instead — the cheap boundary map an agent scans to judge relevance before
// drilling into a load-bearing body. The node count goes to stderr so a large
// fan-out doesn't pollute the bundle; a clipped body's marker goes to stdout
// (see clipBody) since a stderr-only hint is invisible to a caller that drops
// stderr.
func emitHydration(cmd *cobra.Command, nodes []spineNode, tldr bool) {
	w := cmd.OutOrStdout()
	emitManifest(w, nodes)
	for _, n := range nodes {
		fmt.Fprintln(w, closureHeader(n))
		if tldr {
			fmt.Fprint(w, compactBody(n))
			continue
		}
		body, total, clipped := clipBody(n.Body)
		if clipped {
			cmd.PrintErrln(output.BodyClipHint(showBodyLineCap, total, n.Path))
			// a 2>/dev/null caller must still see the cut; the stderr hint alone is invisible to it
			body += "\n" + output.BodyClipMarker(total, n.Path)
		}
		fmt.Fprintln(w, body)
	}
	cmd.PrintErrf("hydrated %d spine node(s)\n", len(nodes))
}

// compactBody renders a node's --tldr digest: its frontmatter (the labels layer —
// title, description, goal, status carry the relevance signal) followed by the
// body's `## TL;DR` section (heading through the text before the next `## `) when
// one exists. Learnings and conventions carry a TL;DR; other types fall back to
// frontmatter alone, which is where its one-line summary already lives.
func compactBody(n spineNode) string {
	var b strings.Builder
	if fm, err := yaml.Marshal(n.FrontMatter); err == nil {
		b.Write(fm)
	}
	if tldr := index.TLDRSection(n.Body); tldr != "" {
		b.WriteString("## TL;DR\n\n")
		b.WriteString(tldr)
		b.WriteByte('\n')
	}
	return b.String()
}

// clipBody truncates body to showBodyLineCap lines, returning the (possibly
// clipped) text, the original line count, and whether it clipped — both
// consumers (emitHydration, injectHydratedContext) append output.BodyClipMarker
// inline on top of it.
func clipBody(body string) (clipped string, total int, wasClipped bool) {
	lines := strings.Split(body, "\n")
	if body == "" || len(lines) <= showBodyLineCap {
		return body, len(lines), false
	}
	return strings.Join(lines[:showBodyLineCap], "\n"), len(lines), true
}

// emitManifest prints the bundle's index ahead of every body. A closure runs to
// thousands of lines, so a caller that pipes hydrate to `head` sees only the first
// node's body and concludes the contracts and conventions below were never
// returned — two reviewers did exactly that on 2026-08-21. The two markers start
// with `=== ` so the block is greppable; the entries deliberately do not, so a
// `=== <type> <id> (status: <s>) ===` scrape still counts each node once. The
// marker is quoted verbatim by the skills that tell agents to read it, so it
// stays a fixed string — `=== end manifest ===` bounds the block, no prose does.
func emitManifest(w io.Writer, nodes []spineNode) {
	fmt.Fprintf(w, "=== hydrate manifest: %d spine node(s) ===\n", len(nodes))
	for _, n := range nodes {
		fmt.Fprintf(w, "  %-14s %s (%s)\n", n.Type, n.ID, nodeStatus(n))
	}
	fmt.Fprintln(w, "=== end manifest ===")
	fmt.Fprintln(w)
}

// closureHeader formats a spine node's bundle header — `=== <type> <id> (status:
// <s>[, empty]) ===`. Shared by the `hydrate` emit and the `build` driver's
// task-body fold.
func closureHeader(n spineNode) string {
	return fmt.Sprintf("=== %s %s (status: %s) ===", n.Type, n.ID, nodeStatus(n))
}

// nodeStatus renders the status the bundle header and the manifest both report:
// `unset` when frontmatter carries none, `, empty` appended when the node has no
// body — which keeps a node with nothing to read distinct from one left unread.
func nodeStatus(n spineNode) string {
	status := n.Status
	if status == "" {
		status = "unset"
	}
	if strings.TrimSpace(n.Body) == "" {
		status += ", empty"
	}
	return status
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
