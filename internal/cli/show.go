package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
)

const showBodyLineCap = 500

// bodyDefault returns whether `anvil show <type>` should include the body when
// the caller passes neither --body nor --no-body. Bounded types where the body
// is small and almost always wanted (inbox, decision, issue, sweep) default to
// true; plan and other unbounded types default to false.
func bodyDefault(t core.Type) bool {
	switch t {
	case core.TypeInbox, core.TypeDecision, core.TypeIssue, core.TypeSweep:
		return true
	default:
		return false
	}
}

func newShowCmd() *cobra.Command {
	var (
		flagJSON     bool
		flagBody     bool
		flagNoBody   bool
		flagValidate bool
		flagWaves    bool
		flagTask     string
	)

	cmd := &cobra.Command{
		Use:     "show <type> <id>",
		Short:   "Display a vault artifact (body included by default for bounded types: inbox, decision, issue, sweep; pass --no-body to suppress, or --body to opt in for plan)",
		Args:    cobra.ExactArgs(2),
		Example: "  anvil show issue issue-42\n  anvil show issue issue-42 --no-body\n  anvil show issue issue-42 --json\n  anvil show plan ANV-142 --body\n  anvil show plan ANV-142 --task T3 --body",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			if flagBody && flagNoBody {
				return fmt.Errorf("--body and --no-body are mutually exclusive")
			}
			includeBody := bodyDefault(t)
			if flagBody {
				includeBody = true
			}
			if flagNoBody {
				includeBody = false
			}
			if flagTask != "" {
				if t != core.TypePlan {
					return fmt.Errorf("--task is only valid for plan artifacts")
				}
				if flagValidate || flagWaves {
					return fmt.Errorf("--task cannot be combined with --validate or --waves")
				}
				return runShowPlanTask(cmd, v, args[1], flagTask, flagJSON, includeBody)
			}
			if t == core.TypePlan && (flagValidate || flagWaves) {
				return runShowPlan(cmd, v, args[1], flagValidate, flagWaves)
			}
			if flagValidate && (t == core.TypeIssue || t == core.TypeMilestone) {
				return runShowValidate(cmd, v, t, args[1], flagJSON)
			}
			return runShow(cmd, v, t, args[1], flagJSON, includeBody)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.Flags().BoolVar(&flagBody, "body", false, "include body (capped at 500 lines); opt-in for plan, default for bounded types")
	cmd.Flags().BoolVar(&flagNoBody, "no-body", false, "exclude body (frontmatter only); overrides per-type default")
	cmd.Flags().BoolVar(&flagValidate, "validate", false, "validate artifact (plan: full DAG; issue/milestone: schema + wikilinks)")
	cmd.Flags().BoolVar(&flagWaves, "waves", false, "render plan waves as mermaid (plan only)")
	cmd.Flags().StringVar(&flagTask, "task", "", "scope output to a single task (plan only; compose with --body for the section text)")
	return cmd
}

type showOutput struct {
	ID             string         `json:"id"`
	Path           string         `json:"path"`
	FrontMatter    map[string]any `json:"frontmatter"`
	Body           *string        `json:"body"`
	BodyTruncated  bool           `json:"body_truncated"`
	BodyLinesTotal int            `json:"body_lines_total"`
}

func runShow(cmd *cobra.Command, v *core.Vault, t core.Type, id string, asJSON, includeBody bool) error {
	path := resolveArtifactPath(v.Root, t, id)
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrArtifactNotFound
		}
		return fmt.Errorf("loading artifact: %w", err)
	}

	body := strings.TrimPrefix(a.Body, "\n")
	bodyLines := strings.Split(body, "\n")
	totalLines := len(bodyLines)
	if body == "" {
		totalLines = 0
	}

	out := showOutput{
		ID:             id,
		Path:           a.Path,
		FrontMatter:    a.FrontMatter,
		BodyLinesTotal: totalLines,
	}

	if includeBody {
		shown := body
		if totalLines > showBodyLineCap {
			shown = strings.Join(bodyLines[:showBodyLineCap], "\n")
			out.BodyTruncated = true
			cmd.PrintErrln(output.BodyClipHint(showBodyLineCap, totalLines, a.Path))
		}
		out.Body = &shown
	}

	w := cmd.OutOrStdout()
	if asJSON {
		b, _ := json.Marshal(out)
		fmt.Fprintln(w, string(b))
		return nil
	}

	emitFrontMatterText(cmd, a.FrontMatter)
	if includeBody && out.Body != nil {
		fmt.Fprintln(w, "---")
		fmt.Fprint(w, *out.Body)
	}
	return nil
}

// resolveArtifactPath maps a CLI (type, id) pair to its on-disk path.
// Singletons accept either the bare project slug or the qualified
// "<type>.<project>" wikilink form; non-singletons compose <Dir>/<id>.md.
func resolveArtifactPath(vaultRoot string, t core.Type, id string) string {
	if t.AllocatesID() {
		return filepath.Join(vaultRoot, t.Dir(), id+".md")
	}
	project := strings.TrimPrefix(id, string(t)+".")
	return filepath.Join(vaultRoot, t.Dir(), project, string(t)+".md")
}

func emitFrontMatterText(cmd *cobra.Command, fm map[string]any) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "---")
	enc, _ := json.MarshalIndent(fm, "", "  ")
	fmt.Fprintln(w, string(enc))
	fmt.Fprintln(w, "---")
}
