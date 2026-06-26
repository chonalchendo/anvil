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

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
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
		flagLinks      string
	)

	cmd := &cobra.Command{
		Use:     "show <type> <id>",
		Short:   "Display a vault artifact (body included by default for bounded types: inbox, decision, issue, sweep; pass --no-body to suppress, or --body to opt in for plan). Also accepts type=skill to print a bundled SKILL.md body.",
		Args:    namedArgs("anvil show <type> <id>", []string{"<type>", "<id>"}, 2, 2),
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
			args[1] = canonicalArtifactID(v, t, args[1])
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
			if flagLinks != "" {
				lt, err := core.ParseType(flagLinks)
				if err != nil {
					return fmt.Errorf("--links: unknown type %q", flagLinks)
				}
				return runShowLinks(cmd, v, t, args[1], lt, flagJSON, flagBody)
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
	cmd.Flags().StringVar(&flagLinks, "links", "", "print wikilink targets of the given type (one per line; --json emits a JSON array; add --body to expand each target's body)")
	return cmd
}

// showOutput is the in-memory shape used by both text and JSON emit paths.
// For JSON, frontmatter fields are flattened onto the top-level envelope so
// callers read `status`, `severity`, etc. with the same idiom they use against
// `anvil list --json` items (no `frontmatter.` indirection). Envelope keys
// (id, path, body, body_truncated, body_lines_total, incoming) take precedence
// on collision with frontmatter keys of the same name.
type showOutput struct {
	ID             string
	Path           string
	FrontMatter    map[string]any
	Body           *string
	BodyTruncated  bool
	BodyLinesTotal int
	Incoming       map[string][]incomingEdge
}

type incomingEdge struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// envelopeKeys is the set of top-level fields show always emits. Frontmatter
// keys colliding with these are shadowed (envelope wins) — keeps the contract
// stable when an artifact ever puts `id`/`path` etc. in its own frontmatter.
var envelopeKeys = map[string]struct{}{
	"id": {}, "path": {}, "body": {},
	"body_truncated": {}, "body_lines_total": {}, "incoming": {},
}

// MarshalJSON produces the flat envelope: frontmatter merged onto the top
// level with envelope fields overriding any colliding keys. Implements
// json.Marshaler so `json.Marshal(out)` (and downstream encoders) emit the
// flat shape transparently. `incoming` is omitted when nil (matches the
// previous `omitempty` behaviour); `body` is serialised as JSON null when
// caller didn't ask for it, preserving the explicit-absence signal.
func (o showOutput) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(o.FrontMatter)+6)
	for k, v := range o.FrontMatter {
		if _, reserved := envelopeKeys[k]; reserved {
			continue
		}
		out[k] = v
	}
	out["id"] = o.ID
	out["path"] = o.Path
	out["body"] = o.Body
	out["body_truncated"] = o.BodyTruncated
	out["body_lines_total"] = o.BodyLinesTotal
	if o.Incoming != nil {
		out["incoming"] = o.Incoming
	}
	return json.Marshal(out)
}

func runShow(cmd *cobra.Command, v *core.Vault, t core.Type, id string, asJSON, includeBody, includeIncoming bool) error {
	path := resolveArtifactPath(v.Root, t, id)
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrArtifactNotFound, id)
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
			// A stale index must not suppress the body — emit the warning to
			// stderr and degrade gracefully (no incoming section) so the body
			// still reaches stdout.
			var s *errfmt.Structured
			if errors.As(err, &s) && s.Code == "index_stale" {
				cmd.PrintErrln(err)
			} else {
				return err
			}
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
		b, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("marshaling show output: %w", err)
		}
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
		return nil, err
	}
	defer db.Close() //nolint:errcheck // close in defer; error not actionable

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

// resolveArtifactPath maps a CLI (type, id) pair to its on-disk path: <Dir>/<id>.md.
func resolveArtifactPath(vaultRoot string, t core.Type, id string) string {
	return filepath.Join(vaultRoot, t.Dir(), id+".md")
}

// canonicalArtifactID normalises a raw id/arg (CLI arg or wikilink target) to
// the on-disk basename for type t: design types keep the "<type>." prefix for
// global uniqueness, issues canonicalise through the shared resolver (qualified,
// project-ordinal, and bare forms), everything else strips the redundant
// "<type>." wikilink prefix.
func canonicalArtifactID(v *core.Vault, t core.Type, raw string) string {
	switch t {
	case core.TypeProductDesign, core.TypeSystemDesign:
		prefix := string(t) + "."
		if !strings.HasPrefix(raw, prefix) {
			return prefix + raw
		}
		return raw
	case core.TypeIssue:
		return core.ResolveIssueArg(v, raw)
	default:
		return strings.TrimPrefix(raw, string(t)+".")
	}
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
			entries, lerr := fs.ReadDir(skills.FS, ".")
			if lerr != nil {
				return fmt.Errorf("unknown skill %q (and failed to list bundled skills: %w)", name, lerr)
			}
			available := make([]string, 0, len(entries))
			for _, e := range entries {
				if e.IsDir() {
					available = append(available, e.Name())
				}
			}
			sort.Strings(available)
			return fmt.Errorf("unknown skill %q; available: %s", name, strings.Join(available, ", "))
		}
		return fmt.Errorf("reading skill %q: %w", name, err)
	}
	fmt.Fprint(cmd.OutOrStdout(), string(data))
	return nil
}

// linkBody is one resolved linked artifact under --links <type> --body: the
// wikilink target id, its frontmatter status (so a non-active design reads as
// advisory), and its capped body.
type linkBody struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Body   string `json:"body"`
}

// runShowLinks filters an artifact's frontmatter for wikilinks of the requested
// type and emits their targets (without [[ ]] brackets) one per line, or as a
// JSON array under --json. With --body each target's body is loaded and printed
// instead (see emitLinkBodies). Empty result exits 0 with no output (text) or [].
func runShowLinks(cmd *cobra.Command, vault *core.Vault, t core.Type, artifactID string, linkType core.Type, asJSON, includeBody bool) error {
	path := resolveArtifactPath(vault.Root, t, artifactID)
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrArtifactNotFound, artifactID)
		}
		return fmt.Errorf("loading artifact: %w", err)
	}

	prefix := "[[" + string(linkType) + "."
	seen := make(map[string]bool)
	targets := make([]string, 0)
	for _, fmval := range a.FrontMatter {
		switch typed := fmval.(type) {
		case string:
			if target, ok := wikilinkTarget(typed, prefix); ok && !seen[target] {
				seen[target] = true
				targets = append(targets, target)
			}
		case []any:
			for _, elem := range typed {
				s, ok := elem.(string)
				if !ok {
					continue
				}
				if target, ok := wikilinkTarget(s, prefix); ok && !seen[target] {
					seen[target] = true
					targets = append(targets, target)
				}
			}
		}
	}
	sort.Strings(targets)

	if includeBody {
		return emitLinkBodies(cmd, vault, linkType, targets, asJSON)
	}

	w := cmd.OutOrStdout()
	if asJSON {
		b, err := json.Marshal(targets)
		if err != nil {
			return fmt.Errorf("marshaling links output: %w", err)
		}
		fmt.Fprintln(w, string(b))
		return nil
	}
	for _, target := range targets {
		fmt.Fprintln(w, target)
	}
	return nil
}

// emitLinkBodies loads each resolved link target's body (capped at
// showBodyLineCap, same as `show <type> --body`) and emits them: a header
// carrying id + status per body in text mode, or a JSON array of linkBody under
// --json. The count goes to stderr (composability: prose off the data stream)
// so a large fan-out is visible, not silent.
func emitLinkBodies(cmd *cobra.Command, v *core.Vault, linkType core.Type, targets []string, asJSON bool) error {
	bodies := make([]linkBody, 0, len(targets))
	for _, target := range targets {
		id := canonicalArtifactID(v, linkType, target)
		a, err := core.LoadArtifact(resolveArtifactPath(v.Root, linkType, id))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrArtifactNotFound, target)
			}
			return fmt.Errorf("loading linked artifact %s: %w", target, err)
		}
		body := strings.TrimPrefix(a.Body, "\n")
		if lines := strings.Split(body, "\n"); body != "" && len(lines) > showBodyLineCap {
			body = strings.Join(lines[:showBodyLineCap], "\n")
			cmd.PrintErrln(output.BodyClipHint(showBodyLineCap, len(lines), a.Path))
		}
		status, _ := a.FrontMatter["status"].(string)
		bodies = append(bodies, linkBody{ID: target, Status: status, Body: body})
	}

	noun := string(linkType)
	if len(bodies) != 1 {
		noun += "s"
	}
	cmd.PrintErrf("%d %s\n", len(bodies), noun)

	w := cmd.OutOrStdout()
	if asJSON {
		b, err := json.Marshal(bodies)
		if err != nil {
			return fmt.Errorf("marshaling link bodies: %w", err)
		}
		fmt.Fprintln(w, string(b))
		return nil
	}
	for _, lb := range bodies {
		status := lb.Status
		if status == "" {
			status = "unset"
		}
		fmt.Fprintf(w, "=== %s (status: %s) ===\n", lb.ID, status)
		fmt.Fprintln(w, lb.Body)
	}
	return nil
}

// wikilinkTarget returns the inner target of a wikilink if s has the form
// [[prefix<rest>]] (non-empty rest, closing ]]). Used to filter frontmatter
// fields by type prefix without re-invoking the full wikilink regex.
func wikilinkTarget(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, "]]") {
		return "", false
	}
	// Strip surrounding [[ and ]] — prefix already begins with [[.
	inner := s[2 : len(s)-2]
	if inner == "" {
		return "", false
	}
	return inner, true
}

func emitFrontMatterText(cmd *cobra.Command, fm map[string]any) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "---")
	enc, _ := json.MarshalIndent(fm, "", "  ")
	fmt.Fprintln(w, string(enc))
	fmt.Fprintln(w, "---")
}
