package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/internal/templates"
)

// templateData holds all variables that frontmatter templates may reference.
// Fields unused by a given type are left at their zero values; templates guard
// conditional fields with {{- if .X }}.
type templateData struct {
	Title            string
	Created          string
	Project          string
	Horizon          string
	TargetDate       string
	SuggestedType    string
	SuggestedProject string
	DecisionMakers   []string
	ID               string
	Slug             string
	Issue            string
	Milestone        string
}

func newCreateCmd() *cobra.Command {
	var (
		flagTitle            string
		flagProject          string
		flagOrdinal          int
		flagTopic            string
		flagDecisionMakers   []string
		flagHorizon          string
		flagTargetDate       string
		flagSuggestedType    string
		flagSuggestedProject string
		flagJSON             bool
		flagIssue            string
		flagMilestone        string
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

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			// Resolve project slug: --project overrides auto-detection.
			// inbox and decision may proceed without a project.
			project := flagProject
			if project == "" && t != core.TypeInbox && t != core.TypeDecision {
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
				if flagMilestone == "" {
					return fmt.Errorf("--milestone is required for plan")
				}
			case core.TypeMilestone:
				if flagOrdinal <= 0 {
					return fmt.Errorf("--ordinal is required for milestone (>0)")
				}
				if flagTargetDate == "" {
					return fmt.Errorf("--target-date is required for milestone")
				}
			case core.TypeDecision:
				if flagTopic == "" {
					return fmt.Errorf("--topic is required for decision")
				}
			}

			// Default decision-makers to [@me] when unset.
			decisionMakers := flagDecisionMakers
			if t == core.TypeDecision && len(decisionMakers) == 0 {
				decisionMakers = []string{"@me"}
			}

			id, err := core.NextID(v, t, core.IDInputs{
				Title:   flagTitle,
				Project: project,
				Topic:   flagTopic,
				Ordinal: flagOrdinal,
			})
			if err != nil {
				return fmt.Errorf("allocating ID: %w", err)
			}

			created := time.Now().UTC().Format("2006-01-02")
			data := templateData{
				Title:            flagTitle,
				Created:          created,
				Project:          project,
				Horizon:          flagHorizon,
				TargetDate:       flagTargetDate,
				SuggestedType:    flagSuggestedType,
				SuggestedProject: flagSuggestedProject,
				DecisionMakers:   decisionMakers,
				ID:               id,
				Slug:             core.Slugify(flagTitle),
				Issue:            flagIssue,
				Milestone:        flagMilestone,
			}

			fm, err := renderFrontMatter(t, data)
			if err != nil {
				return fmt.Errorf("rendering template: %w", err)
			}

			if err := schema.Validate(string(t), fm); err != nil {
				return fmt.Errorf("schema validation: %w", err)
			}

			dir := filepath.Join(v.Root, t.Dir())
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}
			path := filepath.Join(dir, id+".md")
			body := ""
			if t == core.TypePlan {
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
				cmd.Println(path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "artifact title (required)")
	cmd.Flags().StringVar(&flagProject, "project", "", "project slug (overrides auto-detected)")
	cmd.Flags().IntVar(&flagOrdinal, "ordinal", 0, "milestone ordinal (required for milestone)")
	cmd.Flags().StringVar(&flagTopic, "topic", "", "decision topic slug (required for decision)")
	cmd.Flags().StringSliceVar(&flagDecisionMakers, "decision-makers", nil, "comma-separated decision makers (decision only; default [@me])")
	cmd.Flags().StringVar(&flagHorizon, "horizon", "", "plan horizon: week|sprint|month|quarter|year|open (required for plan)")
	cmd.Flags().StringVar(&flagTargetDate, "target-date", "", "target date YYYY-MM-DD (required for plan/milestone)")
	cmd.Flags().StringVar(&flagSuggestedType, "suggested-type", "", "suggested type (inbox only)")
	cmd.Flags().StringVar(&flagSuggestedProject, "suggested-project", "", "suggested project (inbox only)")
	cmd.Flags().StringVar(&flagIssue, "issue", "", "issue wikilink (required for plan)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "milestone wikilink (required for plan)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	_ = cmd.MarkFlagRequired("title")

	return cmd
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
