// Package hydrate walks an issue's methodology spine into one context closure.
// The CLI and the human view share the walk, so both open the same box.
package hydrate

import (
	"fmt"
	"os"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// NotFoundError reports that the issue file does not exist. ID is canonical;
// Input is what the caller passed. The CLI maps it to its own envelope.
type NotFoundError struct {
	ID    string
	Input string
}

func (e *NotFoundError) Error() string { return "issue not found: " + e.ID }

// SpineNode is one resolved artifact in the assembled closure: its type, canonical
// id, frontmatter status (so a non-active design reads as advisory), body, and the
// parsed frontmatter (the --tldr digest renders it in place of the body).
type SpineNode struct {
	Type        core.Type
	ID          string
	Status      string
	Body        string
	Path        string
	FrontMatter map[string]any
}

// BrokenEdge is a declared spine wikilink whose target does not resolve on disk.
// Target carries the full type-qualified wikilink (e.g. milestone.foo.ghost), so
// the edge type needs no separate field.
type BrokenEdge struct {
	Source string // "<type> <id>" of the artifact declaring the edge
	Target string // the type-qualified wikilink target that failed to resolve
}

// Hydration accumulates the assembled closure and any broken edges as the walk
// descends the fixed methodology spine. seen keys the nodes already emitted, so a
// convention reachable by two rails (a component design and a design) enters the bundle
// once — a duplicated body is pure context cost to the reader.
type Hydration struct {
	Nodes  []SpineNode
	Broken []BrokenEdge
	seen   map[string]bool
	// SkippedBodyLinks names the issue body ## Links targets whose type
	// parsed but is not governing (e.g. thread, sibling issue) — reported so
	// the omission is stated, never silent (anvil.0240).
	SkippedBodyLinks []string
}

// Assemble walks the methodology spine from issueID and returns the closure.
// anvil hydrate, walkability and the human view share this one walk, so load order has one owner.
func Assemble(v *core.Vault, issueID string) (*Hydration, error) {
	// Callers hand a canonical id (walkability derives one per file), which may
	// differ from the on-disk basename until the back catalogue is renamed.
	issID, issPath, err := core.ResolveArtifact(v, core.TypeIssue, issueID)
	if err != nil {
		return nil, err
	}
	iss, err := core.LoadArtifact(issPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{ID: issID, Input: issueID}
		}
		return nil, fmt.Errorf("loading issue: %w", err)
	}

	h := &Hydration{
		Nodes: []SpineNode{nodeOf(core.TypeIssue, issueID, iss)},
		seen:  map[string]bool{string(core.TypeIssue) + " " + issueID: true},
	}
	issueSrc := "issue " + issueID

	// issue → milestone → {product-design, system-design} → convention
	for _, mt := range core.LinkTargetsOfType(iss, core.TypeMilestone) {
		ms, err := h.walk(v, issueSrc, core.TypeMilestone, mt)
		if err != nil {
			return nil, err
		}
		if ms == nil {
			continue
		}
		msSrc := "milestone " + mt
		for _, dtype := range []core.Type{core.TypeProductDesign, core.TypeSystemDesign} {
			for _, dt := range core.LinkTargetsOfType(ms, dtype) {
				if err := h.walkDesign(v, msSrc, dtype, dt); err != nil {
					return nil, err
				}
			}
		}
	}

	// issue → component design → {convention, system-design → convention}
	for _, ct := range core.LinkTargetsOfType(iss, core.TypeComponentDesign) {
		c, err := h.walk(v, issueSrc, core.TypeComponentDesign, ct)
		if err != nil {
			return nil, err
		}
		if c == nil {
			continue
		}
		cSrc := "component design " + ct
		if err := h.descendConventions(v, cSrc, c); err != nil {
			return nil, err
		}
		// Walk the component design's forward system-design links; back-links
		// to prefix-retaining types do not resolve as incoming edges. seen
		// dedups a design the milestone path already reached.
		for _, st := range core.LinkTargetsOfType(c, core.TypeSystemDesign) {
			if err := h.walkDesign(v, cSrc, core.TypeSystemDesign, st); err != nil {
				return nil, err
			}
		}
	}

	// issue → prior learnings
	for _, lt := range core.LinkTargetsOfType(iss, core.TypeLearning) {
		if _, err := h.walk(v, issueSrc, core.TypeLearning, lt); err != nil {
			return nil, err
		}
	}

	// issue body `## Links` → the governing artifacts the author deliberately
	// placed there. Unlike the rails above this isn't a fixed spine hop; the
	// type filter is governingBodyLinkTypes (see links_resolve.go).
	bodyTargets, skipped := core.BodyLinksSectionTargets(iss.Body)
	h.SkippedBodyLinks = skipped
	for _, target := range bodyTargets {
		if _, err := h.walk(v, issueSrc, target.Type, target.ID); err != nil {
			return nil, err
		}
	}

	return h, nil
}

// walk resolves target of linkType declared by sourceDesc via forward file
// resolution (target file exists?), never incoming-edge presence — forward
// resolution keeps hydrate independent of index freshness, so a vault whose
// links table predates the canonical-target fix still walks correctly. A
// missing target records a broken edge and returns nil so the walk continues;
// the loaded artifact is returned so the caller can descend into its own links.
func (h *Hydration) walk(v *core.Vault, sourceDesc string, linkType core.Type, target string) (*core.Artifact, error) {
	id, path, err := core.ResolveArtifact(v, linkType, target)
	if err != nil {
		return nil, err
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			h.Broken = append(h.Broken, BrokenEdge{Source: sourceDesc, Target: target})
			return nil, nil
		}
		return nil, fmt.Errorf("loading %s %s: %w", linkType, target, err)
	}
	// ResolveArtifact reports the canonical id — a bare back-catalogue
	// filename must not leak into hydrate's output.
	if key := string(linkType) + " " + id; !h.seen[key] {
		h.seen[key] = true
		h.Nodes = append(h.Nodes, nodeOf(linkType, id, a))
	}
	return a, nil
}

// walkDesign walks one design target, then its convention links.
func (h *Hydration) walkDesign(v *core.Vault, src string, t core.Type, target string) error {
	d, err := h.walk(v, src, t, target)
	if err != nil || d == nil {
		return err
	}
	return h.descendConventions(v, string(t)+" "+target, d)
}

// descendConventions walks an artifact's convention links — the shared last hop of
// both governing rails. A design carries the house style every issue under it must
// obey, so reaching conventions only through a component design left them unreachable for
// any issue whose repo declares no component design.
func (h *Hydration) descendConventions(v *core.Vault, sourceDesc string, a *core.Artifact) error {
	for _, cv := range core.LinkTargetsOfType(a, core.TypeConvention) {
		if _, err := h.walk(v, sourceDesc, core.TypeConvention, cv); err != nil {
			return err
		}
	}
	return nil
}

func nodeOf(t core.Type, id string, a *core.Artifact) SpineNode {
	status, _ := a.FrontMatter["status"].(string)
	return SpineNode{Type: t, ID: id, Status: status, Body: strings.TrimPrefix(a.Body, "\n"), Path: a.Path, FrontMatter: a.FrontMatter}
}
