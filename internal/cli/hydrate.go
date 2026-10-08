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
	"github.com/chonalchendo/anvil/internal/hydrate"
	"github.com/chonalchendo/anvil/internal/index"
)

// newHydrateCmd assembles an issue's methodology-spine context closure into one
// bundle of linked bodies: the issue, its milestone, the milestone's designs, the
// conventions those designs and the issue's component designs govern by, its prior
// learnings, and the governing-type targets named in the issue body's ## Links
// section. A spine edge whose target does not resolve on disk makes the command
// exit non-zero naming it, rather than silently omitting it.
func newHydrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hydrate <issue>",
		Short:   "Assemble an issue's linked-context closure (issue → milestone → designs → conventions, component designs → conventions, learnings, body ## Links) as bodies; a dangling spine edge exits non-zero naming it",
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
			id, path, err := core.ResolveArtifact(v, core.TypeIssue, args[0])
			if err != nil {
				return err
			}
			// Probed here, where the raw arg is still in hand: hydrate.Assemble
			// only ever sees the canonical id.
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return notFoundErr(id, args[0])
			}
			return runHydrate(cmd, v, id, tldr)
		},
	}
	cmd.Flags().Bool("tldr", false, "emit each spine node's frontmatter + ## TL;DR only instead of full bodies — a compact boundary map to scan before drilling into a node with `anvil show <type> <id> --body`")
	return cmd
}

func runHydrate(cmd *cobra.Command, v *core.Vault, issueID string, tldr bool) error {
	h, err := hydrate.Assemble(v, issueID)
	if err != nil {
		return mapHydrateErr(err)
	}
	emitHydration(cmd, h.Nodes, h.SkippedBodyLinks, tldr)
	if len(h.Broken) > 0 {
		return brokenSpineError(h.Broken)
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
func emitHydration(cmd *cobra.Command, nodes []hydrate.SpineNode, skippedBodyLinks []string, tldr bool) {
	w := cmd.OutOrStdout()
	emitManifest(w, nodes, skippedBodyLinks)
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
func compactBody(n hydrate.SpineNode) string {
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
// node's body and concludes the component designs and conventions below were never
// returned — two reviewers did exactly that on 2026-08-21. The two markers start
// with `=== ` so the block is greppable; the entries deliberately do not, so a
// `=== <type> <id> (status: <s>) ===` scrape still counts each node once. The
// marker is quoted verbatim by the skills that tell agents to read it, so it
// stays a fixed string — `=== end manifest ===` bounds the block, no prose does.
// A body ## Links target dropped for a non-governing type is reported on its
// own line immediately after the block (never inside it), so the omission is
// stated rather than silent (anvil.0240) without disturbing the `=== <type>
// <id> (status: <s>) ===` node-header scrape.
func emitManifest(w io.Writer, nodes []hydrate.SpineNode, skippedBodyLinks []string) {
	fmt.Fprintf(w, "=== hydrate manifest: %d spine node(s) ===\n", len(nodes))
	for _, n := range nodes {
		fmt.Fprintf(w, "  %-14s %s (%s)\n", n.Type, n.ID, nodeStatus(n))
	}
	fmt.Fprintln(w, "=== end manifest ===")
	if len(skippedBodyLinks) > 0 {
		fmt.Fprintf(w, "%d body ## Links target(s) not traversed (non-governing type): %s\n",
			len(skippedBodyLinks), strings.Join(skippedBodyLinks, ", "))
	}
	fmt.Fprintln(w)
}

// closureHeader formats a spine node's bundle header — `=== <type> <id> (status:
// <s>[, empty]) ===`. Shared by the `hydrate` emit and the `build` driver's
// task-body fold.
func closureHeader(n hydrate.SpineNode) string {
	return fmt.Sprintf("=== %s %s (status: %s) ===", n.Type, n.ID, nodeStatus(n))
}

// nodeStatus renders the status the bundle header and the manifest both report:
// `unset` when frontmatter carries none, `, empty` appended when the node has no
// body — which keeps a node with nothing to read distinct from one left unread.
func nodeStatus(n hydrate.SpineNode) string {
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
func brokenSpineError(broken []hydrate.BrokenEdge) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%d broken spine edge(s):", len(broken))
	for _, e := range broken {
		fmt.Fprintf(&b, "\n  %s → [[%s]] (target not found)", e.Source, e.Target)
	}
	return errors.New(b.String())
}

// mapHydrateErr turns the walk's typed not-found error into the CLI envelope.
func mapHydrateErr(err error) error {
	var nf *hydrate.NotFoundError
	if errors.As(err, &nf) {
		return notFoundErr(nf.ID, nf.Input)
	}
	return err
}
