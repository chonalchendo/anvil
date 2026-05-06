package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

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
