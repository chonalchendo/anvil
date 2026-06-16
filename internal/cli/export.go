package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "export",
		Short:        "Export vault data in external formats",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newExportTracesCmd())
	return cmd
}

// traceRow is the JSONL record shape emitted by `anvil export traces`. Kept
// intentionally generic — prompt+outcome is the RL/eval minimum; callers that
// need cost / token data can join on task_id in a downstream step.
type traceRow struct {
	Prompt  string `json:"prompt"`
	Outcome string `json:"outcome"`
	TaskID  string `json:"task_id"`
	Model   string `json:"model,omitempty"`
	Effort  string `json:"effort,omitempty"`
}

func newExportTracesCmd() *cobra.Command {
	var (
		flagFormat string
		flagOut    string
	)
	cmd := &cobra.Command{
		Use:   "traces",
		Short: "Export successful build-task traces as an eval/RL dataset",
		Long: `Read successful build-task traces from vault.db and emit them as a
prompt+outcome dataset. Traces are recorded by 'anvil build' after each run.

Only tasks whose outcome is "success" are included. The dataset is an external
artifact — anvil produces it; training is the caller's responsibility.`,
		Example: `  anvil export traces --format jsonl --out /tmp/traces.jsonl
  anvil export traces --format jsonl --out -`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagFormat != "jsonl" {
				return fmt.Errorf("unsupported --format %q: only \"jsonl\" is supported", flagFormat)
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := index.Open(index.DBPath(v.Root))
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable

			traces, err := db.ListSuccessfulTraces()
			if err != nil {
				return fmt.Errorf("listing traces: %w", err)
			}

			var out *os.File
			if flagOut == "" || flagOut == "-" {
				out = os.Stdout
			} else {
				out, err = os.Create(flagOut) //nolint:gosec // path is an explicit user-supplied argument
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer out.Close() //nolint:errcheck // close in defer; error not actionable
			}

			enc := json.NewEncoder(out)
			for _, tr := range traces {
				row := traceRow{
					Prompt:  tr.Prompt,
					Outcome: tr.Outcome,
					TaskID:  tr.TaskID,
					Model:   tr.Model,
					Effort:  tr.Effort,
				}
				if err := enc.Encode(row); err != nil {
					return fmt.Errorf("encoding trace %d: %w", tr.ID, err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "jsonl", "output format (currently only \"jsonl\")")
	cmd.Flags().StringVar(&flagOut, "out", "-", "output file path; \"-\" writes to stdout")
	return cmd
}
