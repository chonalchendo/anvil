package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
	"github.com/chonalchendo/anvil/skills"
)

const showBodyLineCap = 500

func newShowCmd() *cobra.Command {
	var (
		flagJSON       bool
		flagBody       bool
		flagNoBody     bool
		flagValidate   bool
		flagWaves      bool
		flagTask       string
		flagNoIncoming bool
	)

	cmd := &cobra.Command{
		Use:     "show <type> <id>",
		Short:   "Display a vault artifact (body included by default for bounded types: inbox, decision, issue, sweep; pass --no-body to suppress, or --body to opt in for plan). Also accepts type=skill to print a bundled SKILL.md body.",
		Args:    cobra.ExactArgs(2),
		Example: "  anvil show issue issue-42\n  anvil show issue issue-42 --no-body\n  anvil show issue issue-42 --json\n  anvil show plan ANV-142\n  anvil show plan ANV-142 --task T3\n  anvil show plan ANV-142 --task T3 --body\n  anvil show skill capturing-inbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Skills are bundled, not vault artifacts — short-circuit before
			// ParseType so `anvil show skill <name>` reads from the embedded
			// skill bundle rather than failing with "unknown type".
			if args[0] == "skill" {
				return runShowSkill(cmd, args[1])
			}
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			if t.AllocatesID() {
				prefix := string(t) + "."
				if strings.HasPrefix(args[1], prefix) {
					candidate := strings.TrimPrefix(args[1], prefix)
					// Guard: only strip when remainder still contains "." —
					// proves the input is "<type>.<project>.<slug>", not the
					// bare ID "<type>.<project>" where project equals type name.
					if strings.Contains(candidate, ".") {
						args[1] = candidate
					}
				}
			}
			if flagBody && flagNoBody {
				return fmt.Errorf("--body and --no-body are mutually exclusive")
			}
			includeBody := false
			switch t {
			case core.TypeInbox, core.TypeDecision, core.TypeIssue, core.TypeSweep:
				includeBody = true
			}
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
			return runShow(cmd, v, t, args[1], flagJSON, includeBody, !flagNoIncoming)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.Flags().BoolVar(&flagBody, "body", false, "include body (capped at 500 lines); opt-in for plan, default for bounded types")
	cmd.Flags().BoolVar(&flagNoBody, "no-body", false, "exclude body (frontmatter only); overrides per-type default")
	cmd.Flags().BoolVar(&flagValidate, "validate", false, "validate artifact (plan: full DAG; issue/milestone: schema + wikilinks)")
	cmd.Flags().BoolVar(&flagWaves, "waves", false, "render plan waves as mermaid (plan only)")
	cmd.Flags().StringVar(&flagTask, "task", "", "scope output to a single task (plan only; compose with --body for the section text)")
	cmd.Flags().BoolVar(&flagNoIncoming, "no-incoming", false, "suppress the Incoming links section (artifacts whose related[]/etc. point at this one)")
	return cmd
}

type showOutput struct {
	ID             string                    `json:"id"`
	Path           string                    `json:"path"`
	FrontMatter    map[string]any            `json:"frontmatter"`
	Body           *string                   `json:"body"`
	BodyTruncated  bool                      `json:"body_truncated"`
	BodyLinesTotal int                       `json:"body_lines_total"`
	Incoming       map[string][]incomingEdge `json:"incoming,omitempty"`
}

type incomingEdge struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func runShow(cmd *cobra.Command, v *core.Vault, t core.Type, id string, asJSON, includeBody, includeIncoming bool) error {
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

	if includeIncoming {
		incoming, err := loadIncomingEdges(v, id)
		if err != nil {
			return err
		}
		out.Incoming = incoming
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
	emitIncomingText(cmd, out.Incoming)
	if includeBody && out.Body != nil {
		fmt.Fprintln(w, "---")
		fmt.Fprint(w, *out.Body)
	}
	return nil
}

// loadIncomingEdges returns artifacts whose outgoing edges target id, grouped
// by source type. Missing index (vault not initialised) and unindexed sources
// (link target without a backing artifact row) are skipped silently — incoming
// is an additive display surface; degrading gracefully beats failing the whole
// show call. Returns nil when no edges exist so JSON `omitempty` keeps the
// envelope compact.
func loadIncomingEdges(v *core.Vault, id string) (map[string][]incomingEdge, error) {
	db, err := indexForRead(v)
	if err != nil {
		// Surface stale-index errors so callers see the same actionable hint
		// as `anvil link --to` and `anvil list`. Other errors propagate too.
		return nil, err
	}
	defer db.Close()

	rows, err := db.LinksTo(id)
	if err != nil {
		return nil, fmt.Errorf("query incoming edges: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	seen := make(map[string]bool, len(rows))
	grouped := make(map[string][]incomingEdge)
	for _, r := range rows {
		if r.Source == id || seen[r.Source] {
			continue
		}
		seen[r.Source] = true

		row, err := db.GetArtifact(r.Source)
		if err != nil {
			if errors.Is(err, index.ErrArtifactNotInIndex) {
				continue
			}
			return nil, fmt.Errorf("resolve incoming source %s: %w", r.Source, err)
		}
		title := ""
		if a, lerr := core.LoadArtifact(row.Path); lerr == nil {
			title, _ = a.FrontMatter["title"].(string)
		}
		grouped[row.Type] = append(grouped[row.Type], incomingEdge{ID: r.Source, Title: title})
	}
	if len(grouped) == 0 {
		return nil, nil
	}
	for _, edges := range grouped {
		sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	}
	return grouped, nil
}

func emitIncomingText(cmd *cobra.Command, incoming map[string][]incomingEdge) {
	if len(incoming) == 0 {
		return
	}
	w := cmd.OutOrStdout()
	types := make([]string, 0, len(incoming))
	for k := range incoming {
		types = append(types, k)
	}
	sort.Strings(types)
	fmt.Fprintln(w, "Incoming links:")
	for _, typ := range types {
		fmt.Fprintf(w, "  %s:\n", typ)
		for _, e := range incoming[typ] {
			if e.Title != "" {
				fmt.Fprintf(w, "    - %s — %s\n", e.ID, e.Title)
			} else {
				fmt.Fprintf(w, "    - %s\n", e.ID)
			}
		}
	}
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

// runShowSkill prints the embedded SKILL.md body for the named bundled skill.
// Source-of-truth is the binary's embed.FS — same content `anvil install
// skills` deposits to ~/.claude/skills/<name>/SKILL.md — so output is stable
// regardless of whether the user has run install. Unknown names list the
// available skill set so agents can self-correct without grepping the repo.
func runShowSkill(cmd *cobra.Command, name string) error {
	data, err := fs.ReadFile(skills.FS, filepath.Join(name, "SKILL.md"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			available, lerr := listBundledSkills()
			if lerr != nil {
				return fmt.Errorf("unknown skill %q (and failed to list bundled skills: %w)", name, lerr)
			}
			return fmt.Errorf("unknown skill %q; available: %s", name, strings.Join(available, ", "))
		}
		return fmt.Errorf("reading skill %q: %w", name, err)
	}
	fmt.Fprint(cmd.OutOrStdout(), string(data))
	return nil
}

func listBundledSkills() ([]string, error) {
	entries, err := fs.ReadDir(skills.FS, ".")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func emitFrontMatterText(cmd *cobra.Command, fm map[string]any) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "---")
	enc, _ := json.MarshalIndent(fm, "", "  ")
	fmt.Fprintln(w, string(enc))
	fmt.Fprintln(w, "---")
}
