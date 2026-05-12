package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/internal/templates"
)

var validSessionSources = []string{"claude-code", "chatgpt", "claude-web", "cursor", "continue"}

// templateData holds all variables that frontmatter templates may reference.
// Fields unused by a given type are left at their zero values; templates guard
// conditional fields with {{- if .X }}.
type templateData struct {
	Title            string
	Created          string
	Description      string
	Project          string
	SuggestedType    string
	SuggestedProject string
	ID               string
	Slug             string
	Issue            string
	ShortID          string
	Source           string
	SessionID        string
	RetentionUntil   string
	ActiveThread     string
	StartedAt        string
	Breaking         bool
	Scope            string
	Tags             []string
}

func newCreateCmd() *cobra.Command {
	var (
		flagTitle            string
		flagDescription      string
		flagProject          string
		flagTopic            string
		flagSuggestedType    string
		flagSuggestedProject string
		flagSlug             string
		flagJSON             bool
		flagIssue            string
		flagBody             string
		flagBreaking         bool
		flagScope            string
		flagSessionID        string
		flagSource           string
		flagStartedAt        string
		flagActiveThread     string
		flagUpdate           bool
		flagTags             []string
		flagAllowNewFacet    []string
		flagForceNew         bool
	)

	cmd := &cobra.Command{
		Use:   "create <type>",
		Short: "Create a new vault artifact",
		Long:  createLongDescription(),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}

			if t != core.TypeSession && flagTitle == "" {
				return fmt.Errorf("--title is required for %s", t)
			}

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			if t == core.TypeSession {
				return runCreateSession(cmd, v, flagSessionID, flagSource, flagStartedAt, flagActiveThread, flagJSON, flagUpdate)
			}

			// Resolve project slug: --project overrides auto-detection.
			// inbox and decision may proceed without a project.
			project := flagProject
			if project == "" && t != core.TypeInbox && t != core.TypeDecision && t != core.TypeThread && t != core.TypeLearning && t != core.TypeSweep {
				p, err := core.ResolveProject()
				if err != nil {
					if errors.Is(err, core.ErrNoProject) {
						return fmt.Errorf("%s requires a project: pass --project or run from a git repo with a remote", t)
					}
					return fmt.Errorf("resolving project: %w", err)
				}
				project = p.Slug
			}

			// Per-type mandatory flag checks.
			switch t {
			case core.TypePlan:
				if flagIssue == "" {
					return fmt.Errorf("--issue is required for plan")
				}
			case core.TypeDecision:
				if flagTopic == "" {
					return fmt.Errorf("--topic is required for decision")
				}
			case core.TypeSweep:
				if flagScope == "" {
					return fmt.Errorf("--scope is required for sweep")
				}
				if !cmd.Flags().Changed("breaking") {
					return fmt.Errorf("--breaking must be set explicitly for sweep (true or false)")
				}
			}

			var (
				id   string
				path string
			)
			switch {
			// Decisions allocate an ordinal by scanning the vault, so they cannot
			// use DeterministicID's slug-only path.
			case t == core.TypeDecision:
				allocated, err := core.NextID(v, t, core.IDInputs{
					Title:   flagTitle,
					Project: project,
					Topic:   flagTopic,
					Slug:    flagSlug,
				})
				if err != nil {
					return invalidSlugError(flagSlug, err)
				}
				id = allocated
			case t.AllocatesID():
				base, err := core.DeterministicID(t, core.IDInputs{
					Title:   flagTitle,
					Project: project,
					Slug:    flagSlug,
				})
				if err != nil {
					return invalidSlugError(flagSlug, err)
				}
				id = base
			default:
				id = string(t)
			}
			path = t.Path(v.Root, project, id)

			body, err := readBody(cmd, flagBody)
			if err != nil {
				return err
			}

			created := time.Now().UTC().Format("2006-01-02")
			data := templateData{
				Title:            flagTitle,
				Created:          created,
				Description:      flagDescription,
				Project:          project,
				SuggestedType:    flagSuggestedType,
				SuggestedProject: flagSuggestedProject,
				ID:               id,
				Slug:             core.Slugify(flagTitle),
				Issue:            flagIssue,
				Breaking:         flagBreaking,
				Scope:            flagScope,
				Tags:             flagTags,
			}

			fm, err := renderFrontMatter(t, data)
			if err != nil {
				return fmt.Errorf("rendering template: %w", err)
			}
			if len(flagTags) > 0 {
				anyTags := make([]any, 0, len(flagTags))
				for _, s := range flagTags {
					anyTags = append(anyTags, s)
				}
				fm["tags"] = anyTags
			}

			if t != core.TypeDecision {
				if existing, err := core.LoadArtifact(path); err == nil {
					drift := createDrift(t, fm, existing.FrontMatter, body, existing.Body)
					if drift == "" {
						return emitCreateResult(cmd, flagJSON, id, path, statusAlreadyExists, nil)
					}
					if !flagUpdate {
						return formatDriftError(cmd, id, drift, fm, existing.FrontMatter, body, existing.Body)
					}
					// --update path: preserve `created`, then re-run schema + facet
					// checks against the new fm, and re-run plan-validate after save.
					if c, ok := existing.FrontMatter["created"]; ok {
						fm["created"] = c
					}
					if err := schema.Validate(string(t), fm); err != nil {
						return renderSchemaErr(cmd, path, err)
					}
					if err := runFacetCheck(cmd, v, path, fm, flagAllowNewFacet); err != nil {
						return err
					}
					var originalBytes []byte
					if t == core.TypePlan {
						var rerr error
						originalBytes, rerr = os.ReadFile(path)
						if rerr != nil {
							return fmt.Errorf("reading existing plan for rollback: %w", rerr)
						}
					}
					a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
					if err := a.Save(); err != nil {
						return fmt.Errorf("saving artifact: %w", err)
					}
					if t == core.TypePlan {
						p, lerr := core.LoadPlan(path)
						if lerr != nil {
							_ = os.WriteFile(path, originalBytes, 0o644)
							return fmt.Errorf("plan validator: %w", lerr)
						}
						if verr := core.ValidatePlan(p); verr != nil {
							_ = os.WriteFile(path, originalBytes, 0o644)
							return fmt.Errorf("plan validator: %w", verr)
						}
					}
					if err := indexAfterSave(v, a); err != nil {
						return fmt.Errorf("indexing %s: %w", id, err)
					}
					return emitCreateResult(cmd, flagJSON, id, path, statusUpdated, nil)
				} else if !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("checking %s: %w", path, err)
				}
			}

			if err := schema.Validate(string(t), fm); err != nil {
				return renderSchemaErr(cmd, path, err)
			}

			if err := runFacetCheck(cmd, v, path, fm, flagAllowNewFacet); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
			}

			if t == core.TypePlan && body == "" {
				// Seed a ≥200-char body section for T1 so ValidatePlan passes on
				// a freshly-created plan. The repeat produces 316 chars.
				body = "\n## Task: T1\n\n" + strings.Repeat(
					"Replace this with the RED test, expected failure, GREEN sketch, verify+commit. ", 4) + "\n"
			}
			a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}

			if t == core.TypePlan {
				p, lerr := core.LoadPlan(path)
				if lerr != nil {
					_ = os.Remove(path)
					return fmt.Errorf("plan validator: %w", lerr)
				}
				if verr := core.ValidatePlan(p); verr != nil {
					_ = os.Remove(path)
					return fmt.Errorf("plan validator: %w", verr)
				}
			}

			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("indexing %s: %w", id, err)
			}
			var warnings []string
			if !flagForceNew {
				warnings = findNearDuplicates(v, t, project, id)
			}
			return emitCreateResult(cmd, flagJSON, id, path, statusCreated, warnings)
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "artifact title (required)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "one-line summary (1-120 chars, required for spine types)")
	cmd.Flags().StringVar(&flagProject, "project", "", "project slug (overrides auto-detected)")
	cmd.Flags().StringVar(&flagTopic, "topic", "", "decision topic slug (required for decision)")
	cmd.Flags().StringVar(&flagSuggestedType, "suggested-type", "", "suggested type (inbox only)")
	cmd.Flags().StringVar(&flagSuggestedProject, "suggested-project", "", "suggested project (inbox only)")
	cmd.Flags().StringVar(&flagSlug, "slug", "", "override the title-derived slug (must match ^[a-z0-9][a-z0-9-]*$)")
	cmd.Flags().StringVar(&flagIssue, "issue", "", "issue wikilink (required for plan)")
	cmd.Flags().StringVar(&flagBody, "body", "", "artifact body content (or pipe via stdin)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&flagBreaking, "breaking", false, "sweep is breaking (required for sweep, must be explicit)")
	cmd.Flags().StringVar(&flagScope, "scope", "", "sweep scope (required for sweep)")
	cmd.Flags().StringVar(&flagSessionID, "session-id", "", "session UUID (required for session)")
	cmd.Flags().StringVar(&flagSource, "source", "claude-code", "session source (claude-code|chatgpt|claude-web|cursor|continue)")
	cmd.Flags().StringVar(&flagStartedAt, "started-at", "", "RFC3339 session start time (defaults to now)")
	cmd.Flags().StringVar(&flagActiveThread, "active-thread", "", "active thread slug to record in related[]")
	cmd.Flags().BoolVar(&flagUpdate, "update", false, "rewrite existing session artifact on drift")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "comma-separated tag list (e.g. domain/dbt,activity/testing)")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet to suppress novelty gate for (repeatable: domain|activity|pattern)")
	cmd.Flags().BoolVar(&flagForceNew, "force-new", false, "skip the near-duplicate similarity check")

	return cmd
}

// invalidSlugError wraps a ValidateSlug failure with a structured code so
// agents can dispatch on `invalid_slug` instead of parsing the text. Falls
// through unchanged when slug is empty (the caller's error wasn't a slug
// validation failure).
func invalidSlugError(slug string, cause error) error {
	if slug == "" {
		return cause
	}
	return errfmt.NewInvalidSlug(slug, cause)
}

func createLongDescription() string {
	names := make([]string, 0, len(core.AllTypes))
	for _, t := range core.AllTypes {
		names = append(names, string(t))
	}
	return "Create a new vault artifact.\n\nSupported types: " + strings.Join(names, ", ")
}

func runCreateSession(cmd *cobra.Command, v *core.Vault, sessionID, source, startedAt, activeThread string, asJSON, update bool) error {
	if sessionID == "" {
		return fmt.Errorf("--session-id is required for session")
	}
	validSource := false
	for _, s := range validSessionSources {
		if s == source {
			validSource = true
			break
		}
	}
	if !validSource {
		return formatEnumError("--source", source, validSessionSources, "")
	}

	now := time.Now().UTC()
	if startedAt == "" {
		startedAt = now.Format(time.RFC3339)
	}
	created := now.Format("2006-01-02")
	retention := now.AddDate(0, 0, 30).Format("2006-01-02")
	short := sessionID
	if len(short) > 8 {
		short = short[:8]
	}

	data := templateData{
		Created:        created,
		ShortID:        short,
		Source:         source,
		SessionID:      sessionID,
		RetentionUntil: retention,
		ActiveThread:   activeThread,
		StartedAt:      startedAt,
	}

	dir := filepath.Join(v.Root, core.TypeSession.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, sessionID+".md")

	if existing, err := core.LoadArtifact(path); err == nil {
		if !update {
			if drift := sessionDrift(existing.FrontMatter, source, startedAt, activeThread); drift != "" {
				return fmt.Errorf("session %s already exists with different %s; use --update to rewrite", sessionID, drift)
			}
			if asJSON {
				return emitSessionJSON(cmd, sessionID, path, activeThread)
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", path, err)
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
	if err := indexAfterSave(v, a); err != nil {
		return fmt.Errorf("indexing %s: %w", sessionID, err)
	}

	if asJSON {
		return emitSessionJSON(cmd, sessionID, path, activeThread)
	}
	fmt.Fprintln(cmd.OutOrStdout(), path)
	return nil
}

func sessionDrift(fm map[string]any, source, startedAt, activeThread string) string {
	if got, _ := fm["source"].(string); got != source {
		return "source"
	}
	if got, _ := fm["started_at"].(string); got != "" && got != startedAt {
		return "started_at"
	}
	related, _ := fm["related"].([]any)
	want := ""
	if activeThread != "" {
		want = "[[thread." + activeThread + "]]"
	}
	got := ""
	if len(related) > 0 {
		got, _ = related[0].(string)
	}
	if got != want {
		return "active-thread"
	}
	return ""
}

type createStatus string

const (
	statusCreated       createStatus = "created"
	statusAlreadyExists createStatus = "already_exists"
	statusUpdated       createStatus = "updated"
)

func emitCreateResult(cmd *cobra.Command, asJSON bool, id, path string, status createStatus, warnings []string) error {
	if asJSON {
		payload := map[string]any{
			"id":     id,
			"path":   path,
			"status": string(status),
		}
		if len(warnings) > 0 {
			ws := make([]map[string]string, 0, len(warnings))
			for _, w := range warnings {
				ws = append(ws, map[string]string{"kind": "similar", "id": w})
			}
			payload["warnings"] = ws
		}
		out, _ := json.Marshal(payload)
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}
	switch status {
	case statusCreated:
		fmt.Fprintln(cmd.OutOrStdout(), "created: "+path)
	case statusAlreadyExists:
		fmt.Fprintln(cmd.OutOrStdout(), "already_exists: "+path)
	case statusUpdated:
		fmt.Fprintln(cmd.OutOrStdout(), "updated: "+path)
	}
	for _, w := range warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: similar artifact exists: "+w+" (pass --force-new to skip)")
	}
	return nil
}

// createDrift returns the name of the first field that differs between
// fm/body and an existing artifact. Only flag-settable fields are
// compared; fields mutated via 'anvil set' (e.g. status) are ignored
// so retrying create after a status edit isn't drift.
func createDrift(t core.Type, fm, existing map[string]any, body, existingBody string) string {
	scalarFields := []string{"title", "description", "project"}
	switch t {
	case core.TypePlan:
		scalarFields = append(scalarFields, "issue")
	case core.TypeSweep:
		scalarFields = append(scalarFields, "scope", "breaking")
	case core.TypeInbox:
		scalarFields = append(scalarFields, "suggested_type", "suggested_project")
	}
	for _, f := range scalarFields {
		want := fm[f]
		got := existing[f]
		if want == nil && got == nil {
			continue
		}
		if want != got {
			return f
		}
	}
	if !tagsEqual(fm["tags"], existing["tags"]) {
		return "tags"
	}
	if strings.TrimRight(body, "\n\t ") != strings.TrimRight(existingBody, "\n\t ") {
		return "body"
	}
	return ""
}

func tagsEqual(a, b any) bool {
	as := tagSet(a)
	bs := tagSet(b)
	if len(as) != len(bs) {
		return false
	}
	for k := range as {
		if !bs[k] {
			return false
		}
	}
	return true
}

func tagSet(v any) map[string]bool {
	out := map[string]bool{}
	arr, _ := v.([]any)
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out[s] = true
		}
	}
	return out
}

func formatDriftError(_ *cobra.Command, id, field string, fm, existing map[string]any, body, existingBody string) error {
	var existingStr, newStr string
	switch field {
	case "tags":
		existingStr = renderTagArray(existing["tags"])
		newStr = renderTagArray(fm["tags"])
	case "body":
		existingStr = truncateBody(existingBody)
		newStr = truncateBody(body)
	default:
		existingStr = renderScalar(existing[field])
		newStr = renderScalar(fm[field])
	}
	return fmt.Errorf(
		"%w: %s already exists with different %s\n  existing: %s\n  new:      %s\n  retry with --update to overwrite, or use 'anvil set' to edit a single field",
		ErrCreateDrift, id, field, existingStr, newStr,
	)
}

func renderScalar(v any) string {
	if v == nil {
		return `""`
	}
	return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
}

func renderTagArray(v any) string {
	if arr, ok := v.([]any); ok {
		ordered := make([]string, 0, len(arr))
		seen := map[string]bool{}
		for _, e := range arr {
			if s, ok := e.(string); ok && !seen[s] {
				ordered = append(ordered, s)
				seen[s] = true
			}
		}
		return "[" + strings.Join(ordered, ", ") + "]"
	}
	return "[]"
}

func truncateBody(s string) string {
	s = strings.TrimRight(s, "\n\t ")
	if len(s) <= 80 {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q…", s[:80])
}

func runFacetCheck(cmd *cobra.Command, v *core.Vault, path string, fm map[string]any, allowNewFacet []string) error {
	for _, f := range allowNewFacet {
		if !facets.Has(f) {
			return formatEnumError("--allow-new-facet", f, facets.Names(), "")
		}
	}
	allowed := map[string]bool{}
	for _, f := range allowNewFacet {
		allowed[f] = true
	}
	values, gErr := facets.CollectValues(v.Root)
	if gErr != nil {
		return fmt.Errorf("walking vault for facet values: %w", gErr)
	}
	tagsRaw, _ := fm["tags"].([]any)
	tagsStr := make([]string, 0, len(tagsRaw))
	for _, raw := range tagsRaw {
		if s, ok := raw.(string); ok {
			tagsStr = append(tagsStr, s)
		}
	}
	if errs := facets.Check(values, tagsStr, allowed); len(errs) > 0 {
		for _, e := range errs {
			e.Path = path
		}
		printValidationErrors(cmd, errs)
		return ErrSchemaInvalid
	}
	return nil
}

func emitSessionJSON(cmd *cobra.Command, id, path, activeThread string) error {
	related := []string{}
	if activeThread != "" {
		related = []string{"[[thread." + activeThread + "]]"}
	}
	out, err := json.Marshal(map[string]any{"id": id, "path": path, "related": related})
	if err != nil {
		return fmt.Errorf("marshalling json: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

// renderFrontMatter executes the template for t against data and parses the
// result into a map suitable for schema validation and artifact storage.
func renderFrontMatter(t core.Type, data templateData) (map[string]any, error) {
	src, err := templates.FS.ReadFile(string(t) + ".tmpl")
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", t, err)
	}
	tmpl, err := template.New(string(t)).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", t, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", t, err)
	}
	var fm map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &fm); err != nil {
		return nil, fmt.Errorf("parsing rendered YAML: %w", err)
	}
	// yaml.v3 parses YYYY-MM-DD scalars as time.Time; convert them back to
	// date strings so the JSON Schema validator sees a plain string.
	normaliseDates(fm)
	return fm, nil
}

// normaliseDates walks fm and replaces any time.Time value with its
// YYYY-MM-DD string representation. yaml.v3 auto-converts date scalars.
func normaliseDates(fm map[string]any) {
	for k, v := range fm {
		if t, ok := v.(time.Time); ok {
			fm[k] = t.UTC().Format("2006-01-02")
		}
	}
}
