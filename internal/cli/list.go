package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

const defaultListLimit = 10

type listItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Severity    string   `json:"severity,omitempty"`
	Created     string   `json:"created,omitempty"`
	Project     string   `json:"project,omitempty"`
	Milestone   string   `json:"milestone,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Path        string   `json:"path"`
}

type listFilters struct {
	Status, Project, Tag string
	TagsAllOf            []string
	Diataxis, Confidence string
	Severity             string
	Milestone            string
	Since, Until         string
}

// issueSeverityEnum mirrors schemas/issue.schema.json. Kept inline because
// list is the only consumer; promote to schema package on a second use.
var issueSeverityEnum = []string{"low", "medium", "high", "critical"}

func newListCmd() *cobra.Command {
	var (
		flagStatus, flagProject, flagTag string
		flagDiataxis, flagConfidence     string
		flagSeverity                     string
		flagMilestone                    string
		flagSince, flagUntil             string
		flagTags                         []string
		flagJSON                         bool
		flagLimit                        int
		flagReady, flagOrphans           bool
	)

	cmd := &cobra.Command{
		Use:     "list <type>",
		Short:   "List vault artifacts (default: 10 most recent)",
		Args:    cobra.ExactArgs(1),
		Example: "  anvil list issue --status open\n  anvil list plan --since 2026-05-01 --limit 25\n  anvil list decision --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("project") && !t.SupportsProject() {
				return printAndReturn(cmd, errfmt.NewUnsupportedFlagForType(
					"project", string(t), core.TypesSupportingProject(),
					suggestProjectAlternative(t),
				))
			}
			if flagSeverity != "" && !slices.Contains(issueSeverityEnum, flagSeverity) {
				return printAndReturn(cmd, errfmt.NewStructured("bad_flag_value").
					Set("flag", "severity").
					Set("value", flagSeverity).
					Set("allowed", issueSeverityEnum))
			}
			if flagReady || flagOrphans {
				return runListIndexed(cmd, t, flagReady, flagOrphans, listFilters{
					Status: flagStatus, Project: flagProject,
					Severity: flagSeverity, Milestone: flagMilestone,
					Since: flagSince, Until: flagUntil,
				}, flagJSON, flagLimit)
			}
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			tagsAllOf := splitTags(flagTags)
			return runList(cmd, v, t, listFilters{
				Status: flagStatus, Project: flagProject, Tag: flagTag,
				TagsAllOf: tagsAllOf, Diataxis: flagDiataxis,
				Confidence: flagConfidence, Severity: flagSeverity,
				Milestone: flagMilestone,
				Since:     flagSince, Until: flagUntil,
			}, flagJSON, flagLimit)
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "filter by status (exact match)")
	cmd.Flags().StringVar(&flagProject, "project", "", "filter by project (exact match; supported on: "+strings.Join(core.TypesSupportingProject(), ", ")+")")
	cmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag (substring match, single)")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "filter by tags (all-of, exact, comma-separated)")
	cmd.Flags().StringVar(&flagDiataxis, "diataxis", "", "filter by diataxis (exact match)")
	cmd.Flags().StringVar(&flagConfidence, "confidence", "", "filter by confidence (exact match)")
	cmd.Flags().StringVar(&flagSeverity, "severity", "", "filter by severity (exact match: "+strings.Join(issueSeverityEnum, "|")+"; issue only)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "filter by milestone slug (exact match, e.g. anvil.v0-1-polish-dogfood-findings; issue only)")
	cmd.Flags().StringVar(&flagSince, "since", "", "include only artifacts created on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&flagUntil, "until", "", "include only artifacts created on or before YYYY-MM-DD")
	cmd.Flags().IntVar(&flagLimit, "limit", defaultListLimit, "maximum results to return (default 10)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON envelope")
	cmd.Flags().BoolVar(&flagReady, "ready", false, "filter to issues with no unresolved blockers (issue only)")
	cmd.Flags().BoolVar(&flagOrphans, "orphans", false, "filter to artifacts with no incoming wikilinks")
	return cmd
}

func splitTags(raw []string) []string {
	var out []string
	for _, r := range raw {
		for _, t := range strings.Split(r, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// suggestProjectAlternative returns the recommended scoping mechanism for a
// type whose schema rejects `project:`. Surfaced in the unsupported-flag
// error so agents don't re-discover the convention by guessing.
func suggestProjectAlternative(t core.Type) string {
	if t == core.TypeInbox {
		return "inbox items carry suggested_project, not project; list and grep instead"
	}
	return "this type is deliberately cross-project; filter via --tag or --tags"
}

func runList(cmd *cobra.Command, v *core.Vault, t core.Type, f listFilters, asJSON bool, limit int) error {
	paths, err := collectArtifactPaths(v.Root, t)
	if err != nil {
		return err
	}

	var items []listItem
	for _, path := range paths {
		a, err := core.LoadArtifact(path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", filepath.Base(path), err)
		}
		id := listIDFor(t, path)

		status, _ := a.FrontMatter["status"].(string)
		project, _ := a.FrontMatter["project"].(string)
		title, _ := a.FrontMatter["title"].(string)
		description, _ := a.FrontMatter["description"].(string)
		severity, _ := a.FrontMatter["severity"].(string)
		created, _ := a.FrontMatter["created"].(string)
		diataxis, _ := a.FrontMatter["diataxis"].(string)
		confidence, _ := a.FrontMatter["confidence"].(string)
		milestone := milestoneSlug(a.FrontMatter["milestone"])

		if !matchesFilters(f, status, project, diataxis, confidence, severity, milestone, created, a.FrontMatter["tags"]) {
			continue
		}

		items = append(items, listItem{
			ID: id, Type: string(t), Title: title, Description: description,
			Status: status, Severity: severity, Created: created, Project: project,
			Milestone: milestone,
			Tags:      stringTags(a.FrontMatter["tags"]), Path: path,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Created != items[j].Created {
			return items[i].Created > items[j].Created
		}
		return items[i].ID < items[j].ID
	})

	total := len(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return emitList(cmd, items, total, asJSON, t)
}

func matchesFilters(f listFilters, status, project, diataxis, confidence, severity, milestone, created string, tagsRaw any) bool {
	if f.Status != "" && status != f.Status {
		return false
	}
	if f.Project != "" && project != f.Project {
		return false
	}
	if f.Diataxis != "" && diataxis != f.Diataxis {
		return false
	}
	if f.Confidence != "" && confidence != f.Confidence {
		return false
	}
	if f.Severity != "" && severity != f.Severity {
		return false
	}
	if f.Milestone != "" && milestone != f.Milestone {
		return false
	}
	if f.Tag != "" && !hasTagSubstring(tagsRaw, f.Tag) {
		return false
	}
	if len(f.TagsAllOf) > 0 && !hasAllTags(tagsRaw, f.TagsAllOf) {
		return false
	}
	if f.Since != "" && created < f.Since {
		return false
	}
	if f.Until != "" && created > f.Until {
		return false
	}
	return true
}

func emitList(cmd *cobra.Command, items []listItem, total int, asJSON bool, t core.Type) error {
	returned := len(items)
	if asJSON {
		return output.WriteListJSON(cmd.OutOrStdout(), items, total, returned)
	}
	w := cmd.OutOrStdout()
	for _, item := range items {
		// Em-dash marks milestone-less issues so the column stays visible
		// at pick-time scan.
		milestone := item.Milestone
		if milestone == "" {
			milestone = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.ID, item.Status, milestone, firstNonEmpty(item.Description, item.Title))
	}
	suggestions := []string{"--since/--until", "--status", "--tag"}
	if t.SupportsProject() {
		suggestions = append(suggestions, "--project")
	}
	if t == core.TypeIssue {
		suggestions = append(suggestions, "--milestone")
	}
	if hint := output.TruncationHint("most recent", returned, total, suggestions); hint != "" {
		cmd.PrintErrln(hint)
	}
	return nil
}

// milestoneSlug extracts the slug from a milestone wikilink of the form
// `[[milestone.<slug>]]`. Returns "" for missing, non-string, or
// non-conforming values — issues without a milestone are valid, and a
// malformed link should not crash a list.
func milestoneSlug(raw any) string {
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	s = strings.TrimSpace(s)
	const prefix = "[[milestone."
	const suffix = "]]"
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return ""
	}
	return s[len(prefix) : len(s)-len(suffix)]
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func stringTags(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, t := range list {
		if s, ok := t.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// hasTagSubstring reports whether any element of tags (a []any from YAML) contains sub.
func hasTagSubstring(tags any, sub string) bool {
	list, ok := tags.([]any)
	if !ok {
		return false
	}
	for _, tag := range list {
		if s, ok := tag.(string); ok && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func runListIndexed(cmd *cobra.Command, t core.Type, ready, orphans bool, f listFilters, asJSON bool, limit int) error {
	if ready && t != core.TypeIssue {
		e := errfmt.NewUnsupportedForType(string(t), []string{"issue"})
		return printAndReturn(cmd, e)
	}
	v, err := core.ResolveVault()
	if err != nil {
		return fmt.Errorf("resolving vault: %w", err)
	}
	db, err := indexForRead(v)
	if err != nil {
		return err
	}
	defer db.Close()

	qf := index.QueryFilters{
		Status: f.Status, Project: f.Project,
		Since: f.Since, Until: f.Until,
	}
	var rows []index.ArtifactRow
	switch {
	case ready:
		rows, err = db.ListReady(string(t), qf)
	case orphans:
		rows, err = db.ListOrphans(qf)
	}
	if err != nil {
		return err
	}
	// Severity and milestone are not in the index; filter post-load. Load all
	// matching rows before applying the limit so the truncation hint reflects
	// the filtered total, not the pre-filter row count.
	items := make([]listItem, 0, len(rows))
	for _, r := range rows {
		item := listItem{
			ID: r.ID, Type: r.Type, Status: r.Status,
			Project: r.Project, Path: r.Path, Created: r.Created,
		}
		if a, err := core.LoadArtifact(r.Path); err == nil {
			item.Title, _ = a.FrontMatter["title"].(string)
			item.Description, _ = a.FrontMatter["description"].(string)
			item.Severity, _ = a.FrontMatter["severity"].(string)
			item.Milestone = milestoneSlug(a.FrontMatter["milestone"])
			item.Tags = stringTags(a.FrontMatter["tags"])
		}
		if f.Severity != "" && item.Severity != f.Severity {
			continue
		}
		if f.Milestone != "" && item.Milestone != f.Milestone {
			continue
		}
		items = append(items, item)
	}
	total := len(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return emitList(cmd, items, total, asJSON, t)
}

// collectArtifactPaths returns absolute paths of artifacts of type t under
// vaultRoot. Singletons (product-design, system-design) live one directory
// deeper at 05-projects/<project>/<type>.md and are discovered by walking;
// every other type is a flat <Dir>/<id>.md layout.
func collectArtifactPaths(vaultRoot string, t core.Type) ([]string, error) {
	dir := filepath.Join(vaultRoot, t.Dir())
	if t.AllocatesID() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}
		var out []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			out = append(out, filepath.Join(dir, e.Name()))
		}
		return out, nil
	}
	// Singleton: 05-projects/<project>/<type>.md
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	leaf := string(t) + ".md"
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), leaf)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// listIDFor returns the id surfaced in list output. For singletons the id is
// the project slug (the parent dir name); for other types it is the filename
// stem.
func listIDFor(t core.Type, path string) string {
	if !t.AllocatesID() {
		return filepath.Base(filepath.Dir(path))
	}
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// hasAllTags reports whether tags contains every element of want (exact match).
func hasAllTags(tags any, want []string) bool {
	list, ok := tags.([]any)
	if !ok {
		return false
	}
	have := make(map[string]struct{}, len(list))
	for _, tag := range list {
		if s, ok := tag.(string); ok {
			have[s] = struct{}{}
		}
	}
	for _, w := range want {
		if _, ok := have[w]; !ok {
			return false
		}
	}
	return true
}
