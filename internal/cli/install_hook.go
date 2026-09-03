package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/state"
)

func newInstallFireSessionStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "fire-session-start",
		Short:  "Internal: SessionStart hook wrapper (env→flags for create session)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var payload struct {
				SessionID string `json:"session_id"`
			}
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&payload); err != nil {
				return fmt.Errorf("decoding stdin JSON: %w", err)
			}
			if payload.SessionID == "" {
				return fmt.Errorf("stdin JSON missing session_id")
			}
			active, err := state.ReadActiveThread()
			if err != nil {
				return fmt.Errorf("reading active thread: %w", err)
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			startedAt := time.Now().UTC().Format(time.RFC3339)
			return runCreateSession(cmd, v, payload.SessionID, "claude-code", startedAt, active, false, false)
		},
	}
	return cmd
}

// newInstallFireSessionResumeCmd wraps the SessionStart hook fired on
// matcher "resume|compact": it re-anchors the session on its own handoff by
// session id (never the recency window resuming-session uses for a human
// picking up old work — that would leak a parallel session's state into a
// compacted one), prefixed by the resuming-session skill body so the loop
// re-applies its own load procedure without a human invoking the skill.
func newInstallFireSessionResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "fire-session-resume",
		Short:  "Internal: SessionStart (resume|compact) hook wrapper (re-anchor on own handoff)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var payload struct {
				SessionID string `json:"session_id"`
				Source    string `json:"source"`
			}
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&payload); err != nil {
				return fmt.Errorf("decoding stdin JSON: %w", err)
			}
			if payload.SessionID == "" {
				return fmt.Errorf("stdin JSON missing session_id")
			}
			skillBody, err := fs.ReadFile(skills.FS, filepath.Join("resuming-session", "SKILL.md"))
			if err != nil {
				return fmt.Errorf("reading resuming-session skill: %w", err)
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			items, err := collectSessions(v.Root, "")
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprint(w, string(skillBody))

			var own *sessionItem
			for i := range items {
				if items[i].SessionID == payload.SessionID {
					own = &items[i]
					break
				}
			}
			if own == nil || !own.HasHandoff {
				fmt.Fprintf(w, "\n\n(no prior handoff recorded for session %s)\n", payload.SessionID)
				return nil
			}
			a, err := core.LoadArtifact(own.Path)
			if err != nil {
				return fmt.Errorf("loading session file: %w", err)
			}
			fmt.Fprintf(w, "\n\n## This session's own handoff (%s)\n\n%s\n", payload.SessionID, a.Body)
			for _, m := range findClaimMismatches(v, a.Body, payload.SessionID, payload.SessionID) {
				fmt.Fprintf(w, "\nWarning: %s is claimed by session %s, not this one — stand down or reconcile before acting.\n", m.IssueID, m.ClaimSession)
			}
			if payload.Source == "compact" {
				fmt.Fprint(w, "\nThis SessionStart fired after a compaction. Write a fresh handoff via handing-off-session before your next dispatch, so this compaction's summary is not the only record of state.\n")
			}
			return nil
		},
	}
	return cmd
}

// newInstallFirePreCompactCmd wraps the PreCompact hook: it prints
// hookSpecificOutput.additionalContext in the handing-off-session shape
// (milestone frontier, in-flight agent and issue ids, open PRs and their
// review state, next action) so the compaction summary Claude Code produces
// is itself a handoff, reloaded by the resume|compact SessionStart entry.
func newInstallFirePreCompactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "fire-pre-compact",
		Short:  "Internal: PreCompact hook wrapper (handoff-shaped additionalContext)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var payload struct {
				SessionID          string `json:"session_id"`
				Trigger            string `json:"trigger"`
				CustomInstructions string `json:"custom_instructions"`
			}
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&payload); err != nil {
				return fmt.Errorf("decoding stdin JSON: %w", err)
			}
			additionalContext := "Before this compaction summary replaces the window, capture a " +
				"handing-off-session-shaped record: the milestone frontier (active milestone id, " +
				"next open AC), any in-flight agent and issue ids awaiting completion, open PRs and " +
				"their review state, and the next action to take. A fresh SessionStart on resume or " +
				"compact reloads this by session id via `anvil session show <id> --body` — treat this " +
				"compaction as that handoff, not just a summary."
			out := map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":     "PreCompact",
					"additionalContext": additionalContext,
				},
			}
			enc, err := json.Marshal(out)
			if err != nil {
				return fmt.Errorf("marshalling precompact output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(enc))
			return nil
		},
	}
	return cmd
}
