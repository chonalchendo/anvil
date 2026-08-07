package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// isTopicOrdinalType reports whether t mints `<topic>.<NNNN>-<slug>` ids and so
// requires --topic. Topic is the browse key that groups a folder by subject and
// the join key between a thread and the decision it closes into.
func isTopicOrdinalType(t core.Type) bool {
	return t == core.TypeDecision || t == core.TypeThread
}

// resolveCreateIDPath allocates the id and on-disk path for a new artifact.
// Issues use a per-project atomic ordinal (<project>.NNNN.<slug>) and resolve
// their own path; decisions and threads allocate a topic-scoped ordinal;
// everything else is the slug-keyed DeterministicID. Path defaults to the
// type's slug-based location unless the allocator already resolved it (issues).
//
// release frees an issue's ordinal reservation and must be called once the
// create has written its file or failed; it is a no-op for every other type.
func resolveCreateIDPath(v *core.Vault, t core.Type, project, title, topic, slug string) (id, path string, release func(), err error) {
	release = func() {}
	switch t {
	case core.TypeDecision, core.TypeThread:
		id, err = core.NextID(v, t, core.IDInputs{Title: title, Project: project, Topic: topic, Slug: slug})
	case core.TypeIssue:
		id, path, release, err = core.AllocateIssueID(v, project, title, slug)
	default:
		id, err = core.DeterministicID(t, core.IDInputs{Title: title, Project: project, Slug: slug})
	}
	if err != nil {
		return "", "", release, invalidSlugError(slug, err)
	}
	if path == "" {
		// Resolve through ArtifactBasename, not t.Path(v.Root, id): a
		// back-catalogue file under the other filename shape (e.g. a qualified
		// `product-design.<id>.md` for a type that now mints bare) must become
		// the create target so the drift/already-exists check fires instead of
		// forking a duplicate-id sibling.
		path = t.Path(v.Root, core.ArtifactBasename(v, t, id))
	}
	return id, path, release, nil
}

// slugFromIssueLink extracts the slug component from an issue wikilink of
// the form `[[issue.<project>.<slug>]]` or the numbered form
// `[[issue.<project>.NNNN.<slug>]]`. Returns false when the link doesn't
// match the shape or its project disagrees with the plan's project — both
// signal the caller's `--issue` is malformed; falling back to title-derived
// slug surfaces that to the user via the create flow's normal validation.
func slugFromIssueLink(link, project string) (string, bool) {
	s := strings.TrimSpace(link)
	if !strings.HasPrefix(s, "[[") || !strings.HasSuffix(s, "]]") {
		return "", false
	}
	body := s[2 : len(s)-2]
	const prefix = "issue."
	if !strings.HasPrefix(body, prefix) {
		return "", false
	}
	rest := body[len(prefix):]
	dot := strings.IndexByte(rest, '.')
	if dot < 0 || rest[:dot] != project {
		return "", false
	}
	remainder := rest[dot+1:]
	// Numbered format: <ordinal>.<slug> — strip the ordinal segment.
	if core.IsOrdinalOnly(strings.SplitN(remainder, ".", 2)[0]) {
		if dot2 := strings.IndexByte(remainder, '.'); dot2 >= 0 {
			remainder = remainder[dot2+1:]
		}
	}
	return remainder, true
}

// invalidSlugError wraps a ValidateSlug failure with a structured code so
// agents can dispatch on `invalid_slug` instead of parsing the text. Falls
// through unchanged when slug is empty (the caller's error wasn't a slug
// validation failure).
func invalidSlugError(slug string, cause error) error {
	if slug == "" {
		return cause
	}
	return errfmt.NewInvalidSlug(slug, cause)
}

// normalizeMilestone converts a bare slug (e.g. "anvil.v0-1-polish-dogfood-findings")
// to the canonical wikilink form ("[[milestone.anvil.v0-1-polish-dogfood-findings]]")
// so the issue stays reachable under --milestone filters and index edges.
// Already-wrapped values pass through unchanged.
func normalizeMilestone(s string) string {
	if strings.HasPrefix(s, "[[") {
		return s
	}
	return "[[" + core.WikilinkTarget(core.TypeMilestone, s) + "]]"
}

func createLongDescription() string {
	names := make([]string, 0, len(core.AllTypes))
	for _, t := range core.AllTypes {
		names = append(names, string(t))
	}
	return "Create a new vault artifact.\n\n" +
		"Supported types: " + strings.Join(names, ", ") + "\n\n" +
		"Body authoring: pass --body <literal>, --body-file <path>, or --body - " +
		"(reads stdin). The full artifact lands in one call — no follow-up edit.\n\n" +
		"Required body sections: learning bodies need " + strings.Join(core.RequiredLearningSections, " / ") + "; " +
		"issue bodies need " + strings.Join(core.RequiredIssueSections, " / ") + " (in order). " +
		"Faceted tags (domain/, activity/, pattern/) must reuse existing vault values or pass --allow-new-facet. " +
		"Run 'anvil create <type> --show-template' to print the skeleton before composing.\n\n" +
		"Validation: create always validates the frontmatter it just wrote. " +
		"When --body / --body-file / --body - / --from supplies a body, body " +
		"sections and wikilink targets are validated too; a failure rolls back " +
		"the write. Running 'anvil validate <path>' afterward is unnecessary.\n\n" +
		"EXECUTES CODE (issues only): for an issue body, create runs every " +
		"`### Direct` / `### Indirect` bash block in the `## Verification` section " +
		"and judges the issue by their exit status. Those blocks are author-supplied " +
		"shell: they run in the current environment with your privileges, cwd and " +
		"environment variables, and are NOT sandboxed. Verdicts: Indirect must exit " +
		"non-zero (it asserts post-fix behaviour, so a block that already passes " +
		"cannot tell fixed from broken); Direct may exit anything; either block is " +
		"refused on exit 126/127 (unrunnable). Consequences: create is neither " +
		"read-only nor retry-safe — whatever a block does (rebuild a binary, write a " +
		"file, hit a network) persists even when the create is refused and rolled " +
		"back, so a retry re-runs it. Each block is capped at 60s and killed by " +
		"process group. Pass --skip-verify-predicates to opt out."
}

// sectionsForType returns the required body headings for the types that carry
// a scaffold (learning, issue), or nil for the rest. Shared by the no-body
// scaffold path and --show-template so the two can't drift.
func sectionsForType(t core.Type) []string {
	switch t {
	case core.TypeLearning:
		return core.RequiredLearningSections
	case core.TypeIssue:
		return core.RequiredIssueSections
	default:
		return nil
	}
}

// runShowTemplate prints the required body skeleton and tag rules an author
// needs before composing, then exits — moving create's section/facet checks
// from a post-hoc rollback to an up-front affordance. Only learning and issue
// carry a required-section template.
func runShowTemplate(cmd *cobra.Command, t core.Type) error {
	sections := sectionsForType(t)
	if sections == nil {
		return fmt.Errorf("--show-template: no required body template for %s (learning, issue)", t)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, core.ScaffoldSections(sections))
	fmt.Fprintln(w, "# tags: faceted values (domain/, activity/, pattern/) must already exist in the vault —")
	fmt.Fprintln(w, "# run `anvil tags list --prefix domain/` to see them, or pass --allow-new-facet=<facet> to introduce one.")
	return nil
}
