package cli

import (
	"errors"
	"regexp"

	"github.com/chonalchendo/anvil/internal/core"
)

// resumeOutput is the JSON envelope for `anvil session resume`.
//
// NoHandoff is the explicit no-match signal: present (true) only when a
// --project scope matched no handoff. It distinguishes that empty-but-success
// case from a populated single-candidate hit, which always carries a non-empty
// SessionID/Path. resuming-session branches on it to stop rather than load an
// empty body.
//
// ClaimMismatches has no omitempty: its presence (even as an empty array) is
// the signal `resuming-session` and `jq -e 'has("claim_mismatches")'` probe
// for, regardless of which envelope shape resume returns.
//
// SkippedProjectless names handoffs excluded by --project scoping solely for
// lacking a project: stamp (issue.anvil.0233). It lists those newer than the
// returned result, or any at all in the no-match case. Like ClaimMismatches
// it has no omitempty: presence even when empty is the probe signal.
type resumeOutput struct {
	SessionID          string          `json:"session_id"`
	Path               string          `json:"path"`
	Objective          string          `json:"objective,omitempty"`
	Project            string          `json:"project,omitempty"`
	Body               string          `json:"body"`
	Walked             int             `json:"walked"`
	NoHandoff          bool            `json:"no_handoff,omitempty"`
	Candidates         []sessionItem   `json:"candidates,omitempty"`
	ClaimMismatches    []claimMismatch `json:"claim_mismatches"`
	SkippedProjectless []sessionItem   `json:"skipped_projectless"`
}

// claimMismatch names an issue referenced by a resumed handoff whose
// claim_session frontmatter belongs to a different, still-live session — the
// signal that another session may already be acting on it.
type claimMismatch struct {
	IssueID      string `json:"issue_id"`
	ClaimSession string `json:"claim_session"`
}

// issueRefRe matches a plausible issue-id token in handoff prose: an optional
// "issue." prefix, a project slug, a numbered ordinal, and an optional slug
// tail — e.g. "burgh.0317" or "issue.anvil.0175.foo-bar". Group 2 is the
// token. The leading (^|[^.a-z0-9-]) delimiter — rather than \b — refuses a
// dot-preceded start, so a sibling-type wikilink like [[plan.smoke.0317]] is
// not re-read as issue "smoke.0317". Remaining false positives (e.g. a
// version string) are harmless: resolution against the vault below simply
// finds nothing and is skipped.
var issueRefRe = regexp.MustCompile(`(^|[^.a-z0-9-])((?:issue\.)?[a-z][a-z0-9-]*\.[0-9]{3,6}(?:\.[a-z0-9-]+)*)\b`)

// findClaimMismatches scans body for issue references and returns those still
// in-progress whose claim_session belongs to a third session — neither
// currentSessionID (the resuming session already owns the claim) nor
// authorSessionID (the handoff's author handing its own claim forward, the
// normal ownership-transfer case). Only in-progress issues are considered:
// resolve does not clear claim_session, so a stale claim on completed work
// must not warn. Best-effort: an id that doesn't resolve to a vault issue is
// silently skipped. A caller with no resolvable session id (headless/cron,
// no CLAUDE_CODE_SESSION_ID) gets the empty slice — every claimed issue
// would otherwise warn.
func findClaimMismatches(v *core.Vault, body, currentSessionID, authorSessionID string) []claimMismatch {
	out := []claimMismatch{}
	if currentSessionID == "" {
		return out
	}
	seen := map[string]bool{}
	for _, m := range issueRefRe.FindAllStringSubmatch(body, -1) {
		ids := []string{}
		id, err := core.ResolveIssueArg(v, m[2])
		var amb *core.AmbiguousOrdinalError
		switch {
		case errors.As(err, &amb):
			// A colliding ordinal names every member; skipping would blind the
			// guard on exactly the issues a sync just doubled.
			ids = amb.Candidates
		case err == nil:
			ids = append(ids, id)
		}
		for _, id := range ids {
			basename := core.ArtifactBasename(v, core.TypeIssue, id)
			if seen[basename] {
				continue
			}
			seen[basename] = true
			a, err := core.LoadArtifact(core.TypeIssue.Path(v.Root, basename))
			if err != nil {
				continue
			}
			if status, _ := a.FrontMatter["status"].(string); status != "in-progress" {
				continue
			}
			claim, _ := a.FrontMatter["claim_session"].(string)
			if claim == "" || claim == currentSessionID || claim == authorSessionID {
				continue
			}
			out = append(out, claimMismatch{IssueID: id, ClaimSession: claim})
		}
	}
	return out
}
