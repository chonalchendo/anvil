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
	)

	cmd := &cobra.Command{
		Use:   "create <type>",
		Short: "Create a new vault artifact",
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

			var id string
			if t.AllocatesID() {
				allocated, err := core.NextID(v, t, core.IDInputs{
					Title:   flagTitle,
					Project: project,
					Topic:   flagTopic,
				})
				if err != nil {
					return fmt.Errorf("allocating ID: %w", err)
				}
				id = allocated
			} else {
				id = string(t)
			}

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

			path := t.Path(v.Root, project, id)
			if err := schema.Validate(string(t), fm); err != nil {
				return renderSchemaErr(cmd, path, err)
			}

			for _, f := range flagAllowNewFacet {
				if !facets.Has(f) {
					return formatEnumError("--allow-new-facet", f, facets.Names(), "")
				}
			}
			allowed := map[string]bool{}
			for _, f := range flagAllowNewFacet {
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
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
			}
			if !t.AllocatesID() {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%s for project %q already exists at %s", t, project, path)
				} else if !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("checking %s: %w", path, err)
				}
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

			if flagJSON {
				out, _ := json.Marshal(map[string]string{"id": id, "path": path})
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "artifact title (required)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "one-line summary (1-120 chars, required for spine types)")
	cmd.Flags().StringVar(&flagProject, "project", "", "project slug (overrides auto-detected)")
	cmd.Flags().StringVar(&flagTopic, "topic", "", "decision topic slug (required for decision)")
	cmd.Flags().StringVar(&flagSuggestedType, "suggested-type", "", "suggested type (inbox only)")
	cmd.Flags().StringVar(&flagSuggestedProject, "suggested-project", "", "suggested project (inbox only)")
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

	return cmd
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
	if err := (&core.Artifact{Path: path, FrontMatter: fm}).Save(); err != nil {
		return fmt.Errorf("saving artifact: %w", err)
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
