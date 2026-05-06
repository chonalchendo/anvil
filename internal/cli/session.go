package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/internal/state"
)

var validSources = map[string]bool{
	"claude-code": true,
	"chatgpt":     true,
	"claude-web":  true,
	"cursor":      true,
	"continue":    true,
}

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage session artifacts",
	}
	cmd.AddCommand(newSessionEmitCmd())
	return cmd
}

func newSessionEmitCmd() *cobra.Command {
	var (
		flagSessionID string
		flagSource    string
		flagJSON      bool
		flagFromStdin bool
	)

	cmd := &cobra.Command{
		Use:   "emit",
		Short: "Emit a session artifact for a starting Claude Code session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagFromStdin && flagSessionID != "" {
				return fmt.Errorf("--from-stdin and --session-id are mutually exclusive")
			}
			if flagFromStdin {
				var payload struct {
					SessionID string `json:"session_id"`
				}
				if err := json.NewDecoder(cmd.InOrStdin()).Decode(&payload); err != nil {
					return fmt.Errorf("decoding stdin JSON: %w", err)
				}
				if payload.SessionID == "" {
					return fmt.Errorf("stdin JSON missing session_id")
				}
				flagSessionID = payload.SessionID
			}
			if flagSessionID == "" {
				return fmt.Errorf("--session-id is required and must be non-empty (or use --from-stdin)")
			}
			if !validSources[flagSource] {
				return fmt.Errorf("unknown --source %q", flagSource)
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			dir := filepath.Join(v.Root, core.TypeSession.Dir())
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}
			path := filepath.Join(dir, flagSessionID+".md")

			active, err := state.ReadActiveThread()
			if err != nil {
				return fmt.Errorf("reading active thread: %w", err)
			}

			if _, err := os.Stat(path); err == nil {
				if flagJSON {
					return emitSessionJSON(cmd, flagSessionID, path, active)
				}
				return nil
			}

			now := time.Now().UTC()
			created := now.Format("2006-01-02")
			retention := now.AddDate(0, 0, 30).Format("2006-01-02")
			short := flagSessionID
			if len(short) > 8 {
				short = short[:8]
			}

			data := templateData{
				Created:        created,
				ShortID:        short,
				Source:         flagSource,
				SessionID:      flagSessionID,
				RetentionUntil: retention,
				ActiveThread:   active,
			}
			fm, err := renderFrontMatter(core.TypeSession, data)
			if err != nil {
				return fmt.Errorf("rendering template: %w", err)
			}
			if err := schema.Validate(string(core.TypeSession), fm); err != nil {
				return fmt.Errorf("schema validation: %w", err)
			}
			a := &core.Artifact{Path: path, FrontMatter: fm}
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}

			if flagJSON {
				return emitSessionJSON(cmd, flagSessionID, path, active)
			}
			cmd.Println(path)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagSessionID, "session-id", "", "Claude session UUID (required unless --from-stdin)")
	cmd.Flags().StringVar(&flagSource, "source", "claude-code", "session source (claude-code|chatgpt|claude-web|cursor|continue)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&flagFromStdin, "from-stdin", false, "read SessionStart JSON payload from stdin (Claude Code hook mode)")
	return cmd
}
