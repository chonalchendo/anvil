package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

const resumeAmbiguityWindowSecs = 600

func newSessionResumeCmd() *cobra.Command {
	var flagJSON bool
	var flagProject string
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Return the most-recent handoff, disambiguating when ≥2 landed within the 10-min ambiguity window",
		Long: `Return the most-recent handoff, disambiguating when ≥2 landed within the 10-min ambiguity window.

--json emits one of four envelope shapes:
  single   {session_id, path, objective, project, body, walked}  — one handoff; load body
  multi    {walked, candidates: [...]}                           — ≥2 in window; disambiguate, body empty
  no-match {walked, no_handoff: true}                            — --project matched nothing (exit 0)
  error    (exit 1)                                              — no handoff anywhere (unscoped)
all shapes also carry claim_mismatches: [{issue_id, claim_session}] (empty when none)
--project additionally surfaces skipped_projectless: [...] — newer handoffs excluded only for lacking a project: stamp`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			all, err := collectSessions(v.Root, "")
			if err != nil {
				return err
			}
			items := all
			if flagProject != "" {
				items = []sessionItem{}
				for _, it := range all {
					if it.Project == flagProject {
						items = append(items, it)
					}
				}
			}

			// walked = number of leading stubs (no handoff) before the first handoff.
			walked := 0
			firstHandoffIdx := -1
			for i, it := range items {
				if it.HasHandoff {
					firstHandoffIdx = i
					walked = i
					break
				}
			}
			if firstHandoffIdx == -1 {
				// Scoped to a project with --json: no-match is exit 0 with an
				// explicit no_handoff signal, so callers branch on the payload
				// rather than error-handling the exit code or guessing from
				// empty strings.
				if flagProject != "" {
					skipped := skippedProjectlessHandoffs(all, "")
					if flagJSON {
						return writeJSON(cmd, resumeOutput{Walked: walked, NoHandoff: true, ClaimMismatches: []claimMismatch{}, SkippedProjectless: skipped})
					}
					if len(skipped) > 0 {
						return fmt.Errorf("no prior handoff found for project %q, but %d project-less handoff(s) exist (newest: %s, %s) — run `anvil session list` to check them", flagProject, len(skipped), shortID(skipped[0].SessionID), skipped[0].Modified)
					}
					return fmt.Errorf("no prior handoff found for project %q", flagProject)
				}
				return fmt.Errorf("no prior handoff found — no session file under the vault has a non-empty body")
			}

			newestHandoff := items[firstHandoffIdx]

			// Collect all handoffs in the ambiguity window relative to the newest.
			newestTime, err := time.Parse(time.RFC3339, newestHandoff.Modified)
			if err != nil {
				return fmt.Errorf("parsing modified time %q: %w", newestHandoff.Modified, err)
			}
			candidates := []sessionItem{}
			for _, it := range items {
				if !it.HasHandoff {
					continue
				}
				t, err := time.Parse(time.RFC3339, it.Modified)
				if err != nil {
					continue
				}
				if newestTime.Sub(t).Seconds() <= resumeAmbiguityWindowSecs {
					candidates = append(candidates, it)
				}
			}

			skipped := []sessionItem{}
			if flagProject != "" {
				skipped = skippedProjectlessHandoffs(all, newestHandoff.Modified)
			}
			warnSkipped := func() {
				if len(skipped) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %d newer project-less handoff(s) exist outside project %q scope (newest: %s, %s) — run `anvil session list` to check them.\n", len(skipped), flagProject, shortID(skipped[0].SessionID), skipped[0].Modified)
				}
			}

			if len(candidates) > 1 {
				// Return the candidate list for the caller to disambiguate.
				out := resumeOutput{
					Walked:             walked,
					Candidates:         candidates,
					Body:               "",
					ClaimMismatches:    []claimMismatch{},
					SkippedProjectless: skipped,
				}
				if flagJSON {
					return writeJSON(cmd, out)
				}
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, "Multiple recent handoffs in the ambiguity window:")
				for i, c := range candidates {
					fmt.Fprintf(w, "  %d) %s  %s  %s\n", i+1, shortID(c.SessionID), c.Modified, firstNonEmpty(c.Objective, c.Title))
				}
				fmt.Fprintln(w, "Use `anvil session show <session_id> --body` to load a specific handoff.")
				warnSkipped()
				return nil
			}

			chosen := candidates[0]
			a, err := core.LoadArtifact(chosen.Path)
			if err != nil {
				return fmt.Errorf("loading session file: %w", err)
			}
			currentSessionID, _, _, _ := resolveCurrentSession()
			out := resumeOutput{
				SessionID:          chosen.SessionID,
				Path:               chosen.Path,
				Objective:          chosen.Objective,
				Project:            chosen.Project,
				Body:               a.Body,
				Walked:             walked,
				ClaimMismatches:    findClaimMismatches(v, a.Body, currentSessionID, chosen.SessionID),
				SkippedProjectless: skipped,
			}
			if flagJSON {
				return writeJSON(cmd, out)
			}
			for _, m := range out.ClaimMismatches {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s is claimed by session %s, not this one — stand down or reconcile before acting.\n", m.IssueID, m.ClaimSession)
			}
			warnSkipped()
			fmt.Fprint(cmd.OutOrStdout(), a.Body)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON object")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter candidates to sessions whose project matches this value")
	return cmd
}

// skippedProjectlessHandoffs filters all (unscoped, newest-first) to handoffs
// lacking a project: stamp and newer than cutoff (RFC3339; "" matches any).
// --project scoping hides these, so resume surfaces them rather than silently
// walking past a newer, unstamped handoff (issue.anvil.0233).
func skippedProjectlessHandoffs(all []sessionItem, cutoff string) []sessionItem {
	skipped := []sessionItem{}
	for _, it := range all {
		if !it.HasHandoff || it.Project != "" {
			continue
		}
		if cutoff != "" && it.Modified <= cutoff {
			continue
		}
		skipped = append(skipped, it)
	}
	return skipped
}
