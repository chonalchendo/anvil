package cli

import (
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// resolveCreateIDPath allocates the id and on-disk path for a new artifact.
// Issues use a per-project atomic ordinal (<project>.NNNN.<slug>) and resolve
// their own path; decisions allocate a topic-scoped ordinal; everything else
// is the slug-keyed DeterministicID. Path defaults to the type's slug-based
// location unless the allocator already resolved it (issues).
func resolveCreateIDPath(v *core.Vault, t core.Type, project, title, topic, slug string) (id, path string, err error) {
	switch {
	case t == core.TypeDecision:
		id, err = core.NextID(v, t, core.IDInputs{Title: title, Project: project, Topic: topic, Slug: slug})
	case t == core.TypeIssue:
		id, path, err = core.AllocateIssueID(v, project, title, slug)
	case t.AllocatesID():
		id, err = core.DeterministicID(t, core.IDInputs{Title: title, Project: project, Slug: slug})
	default:
		id = string(t)
	}
	if err != nil {
		return "", "", invalidSlugError(slug, err)
	}
	if path == "" {
		path = t.Path(v.Root, project, id)
	}
	return id, path, nil
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
	return "[[milestone." + s + "]]"
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
		"Validation: create always validates the frontmatter it just wrote. " +
		"When --body / --body-file / --body - / --from supplies a body, body " +
		"sections and wikilink targets are validated too; a failure rolls back " +
		"the write. Running 'anvil validate <path>' afterward is unnecessary."
}
