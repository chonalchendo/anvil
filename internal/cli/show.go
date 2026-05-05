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

const showFullLineCap = 500

func newShowCmd() *cobra.Command {
	var (
		flagJSON     bool
		flagFull     bool
		flagValidate bool
		flagWaves    bool
	)

	cmd := &cobra.Command{
		Use:     "show <type> <id>",
		Short:   "Display a vault artifact (frontmatter-only by default)",
		Args:    cobra.ExactArgs(2),
		Example: "  anvil show issue issue-42\n  anvil show plan plan-7 --full\n  anvil show issue issue-42 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			if t == core.TypePlan && (flagValidate || flagWaves) {
				return runShowPlan(cmd, v, args[1], flagValidate, flagWaves)
			}
			if flagValidate && (t == core.TypeIssue || t == core.TypeMilestone) {
				return runShowValidate(cmd, v, t, args[1], flagJSON)
			}
			return runShow(cmd, v, t, args[1], flagJSON, flagFull)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.Flags().BoolVar(&flagFull, "full", false, "include body (capped at 500 lines)")
	cmd.Flags().BoolVar(&flagValidate, "validate", false, "validate artifact (plan: full DAG; issue/milestone: schema + wikilinks)")
	cmd.Flags().BoolVar(&flagWaves, "waves", false, "render plan waves as mermaid (plan only)")
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

func runShow(cmd *cobra.Command, v *core.Vault, t core.Type, id string, asJSON, full bool) error {
	path := filepath.Join(v.Root, t.Dir(), id+".md")
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

	if full {
		shown := body
		if totalLines > showFullLineCap {
			shown = strings.Join(bodyLines[:showFullLineCap], "\n")
			out.BodyTruncated = true
			cmd.PrintErrln(output.BodyClipHint(showFullLineCap, totalLines, a.Path))
		}
		out.Body = &shown
	}

	if asJSON {
		b, _ := json.Marshal(out)
		cmd.Println(string(b))
		return nil
	}

	emitFrontMatterText(cmd, a.FrontMatter)
	if full && out.Body != nil {
		cmd.Println("---")
		cmd.Print(*out.Body)
	}
	return nil
}

func emitFrontMatterText(cmd *cobra.Command, fm map[string]any) {
	cmd.Println("---")
	enc, _ := json.MarshalIndent(fm, "", "  ")
	cmd.Println(string(enc))
	cmd.Println("---")
}
