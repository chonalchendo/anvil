package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// templateData holds all variables that frontmatter templates may reference.
// Fields unused by a given type are left at their zero values; templates guard
// conditional fields with {{- if .X }}.
type templateData struct {
	Title            string
	Created          string
	Description      string
	Goal             string
	Project          string
	SuggestedType    string
	SuggestedProject string
	ID               string
	Slug             string
	ShortID          string
	Source           string
	SessionID        string
	RetentionUntil   string
	ActiveThread     string
	StartedAt        string
	Breaking         bool
	Scope            string
	Kind             string
	Tags             []string
}

func newCreateCmd() *cobra.Command {
	var (
		flagTitle            string
		flagDescription      string
		flagGoal             string
		flagProject          string
		flagTopic            string
		flagSuggestedType    string
		flagSuggestedProject string
		flagSlug             string
		flagJSON             bool
		flagBody             string
		flagBodyFile         string
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
		flagSeverity         string
		flagMilestone        string
		flagAcceptance       []string
		flagKind             string
		flagShowTemplate     bool
	)

	cmd := &cobra.Command{
		Use:   "create <type>",
		Short: "Create a new vault artifact",
		Long:  createLongDescription(),
		Args:  namedArgs("anvil create <type>", []string{"<type>"}, 1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
			}

			// --show-template surfaces the required body skeleton + tag rules
			// before composing, so create's section/facet checks don't land as
			// a post-hoc rollback. Short-circuits ahead of --title and vault
			// resolution: it touches nothing.
			if flagShowTemplate {
				return runShowTemplate(cmd, t)
			}

			// Stop --project from silently no-op'ing on types whose schema rejects
			// `project:`. Inbox is the documented exception: its schema has
			// `suggested_project`, so we alias internally rather than error,
			// matching the AC. Other unsupported types (session, sweep, thread)
			// fall through to the unsupported_flag_for_type envelope — the same
			// precedent `anvil list <type> --project` already uses.
			if cmd.Flags().Changed("project") && !t.SupportsProject() {
				if t == core.TypeInbox {
					if !cmd.Flags().Changed("suggested-project") {
						_ = cmd.Flags().Set("suggested-project", flagProject)
					}
					flagProject = ""
				} else {
					return printAndReturn(cmd, errfmt.NewUnsupportedFlagForType(
						"project", string(t), core.TypesSupportingProject(),
						"this type is deliberately cross-project; omit --project",
					))
				}
			}

			// Title presence and field caps need no vault, so they fail fast before
			// vault/project resolution (a missing project must not mask a cap
			// overage). preResolutionRefusal reports every violation in one
			// rejection, as text or as the --json envelope. Session and design types
			// do not derive their ID from the title, so --title is optional for them.
			noTitleRequired := t == core.TypeSession || t == core.TypeProductDesign || t == core.TypeSystemDesign
			missingTitle := !noTitleRequired && flagTitle == ""
			capViolations := checkFieldCaps(t, flagDescription, flagGoal)
			if missingTitle || len(capViolations) > 0 {
				return preResolutionRefusal(cmd, flagJSON, t, missingTitle, capViolations)
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
			if project == "" && t != core.TypeInbox && t != core.TypeDecision && t != core.TypeThread && t != core.TypeLearning && t != core.TypeSweep && t != core.TypeConvention {
				p, err := core.ResolveProject()
				if err != nil {
					if errors.Is(err, core.ErrVaultCheckout) {
						return fmt.Errorf("%s requires a project: cwd is the vault checkout, not a project repo — rerun with --project <slug> (anvil project list)", t)
					}
					if errors.Is(err, core.ErrNoProject) {
						return fmt.Errorf("%s requires a project: pass --project or run from a git repo with a remote", t)
					}
					return fmt.Errorf("resolving project: %w", err)
				}
				project = p.Slug
			}

			// Per-type required-flag checks, two tiers — see
			// collectPreValidationErrors.
			preValidationErrors := collectPreValidationErrors(cmd, t, flagTopic)

			// Derive description from title when omitted for spine types that
			// require it, mirroring promote's single-step stub behaviour (see
			// promote.go: Description: title). The author refines via anvil set.
			if flagDescription == "" && (t == core.TypeIssue || t == core.TypeMilestone || t == core.TypeConvention) {
				flagDescription = flagTitle
			}

			slugDefault := flagSlug

			// A missing --topic blocks ID allocation entirely on the
			// topic-ordinal types (decision, thread), so resolution is skipped
			// and id/path stay "": every violation in the block carries the same
			// empty path, and nothing is written — preValidationErrors is
			// non-empty, so validateBeforeCreate rejects before any save.
			// On every other type the resolved path is stamped onto the
			// collected errors so the whole block agrees on one path value.
			var id, path string
			if !isTopicOrdinalType(t) || flagTopic != "" {
				var err error
				var release func()
				id, path, release, err = resolveCreateIDPath(v, t, project, flagTitle, flagTopic, slugDefault)
				// Held until create returns: the ordinal must stay reserved
				// across body validation, which runs the issue's verification
				// blocks and can take minutes.
				defer release()
				if err != nil {
					return err
				}
				for _, e := range preValidationErrors {
					e.Path = path
				}
			}

			// Design types don't require --title; fall back to the id so the
			// schema's minLength constraint is satisfied without a user-supplied
			// title. The author refines via anvil set.
			if (t == core.TypeProductDesign || t == core.TypeSystemDesign) && flagTitle == "" {
				flagTitle = id
			}
			if flagDescription == "" && (t == core.TypeProductDesign || t == core.TypeSystemDesign) {
				flagDescription = id
			}

			var body string
			// userAuthoredBody flags whether the agent supplied body content
			// (via --body, --body-file, --body -, or piped stdin). When
			// true, validation runs body checks (section shape + wikilink
			// resolution). When false, the body is a CLI-generated stub and only
			// the frontmatter is validated.
			var userAuthoredBody bool
			body, err = readBody(cmd, flagBody, flagBodyFile)
			if err != nil {
				return err
			}
			userAuthoredBody = cmd.Flags().Changed("body") || cmd.Flags().Changed("body-file") || body != ""
			if body == "" && !cmd.Flags().Changed("body") && !cmd.Flags().Changed("body-file") {
				body = core.ScaffoldSections(sectionsForType(t))
			}

			created := time.Now().UTC().Format("2006-01-02")
			data := templateData{
				Title:            flagTitle,
				Created:          created,
				Description:      flagDescription,
				Goal:             flagGoal,
				Project:          project,
				SuggestedType:    flagSuggestedType,
				SuggestedProject: flagSuggestedProject,
				ID:               id,
				Slug:             core.Slugify(flagTitle),
				Breaking:         flagBreaking,
				Scope:            flagScope,
				Kind:             flagKind,
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
			if flagSeverity != "" && t == core.TypeIssue {
				fm["severity"] = flagSeverity
			}
			if flagMilestone != "" && t == core.TypeIssue {
				fm["milestone"] = normalizeMilestone(flagMilestone)
			}
			if len(flagAcceptance) > 0 && (t == core.TypeIssue || t == core.TypeMilestone) {
				anyAcc := make([]any, len(flagAcceptance))
				for i, s := range flagAcceptance {
					anyAcc[i] = s
				}
				fm["acceptance"] = anyAcc
			}

			// Templates render flag-backed schema-required scalars
			// unconditionally, so an unset flag lands as ""/null. Drop those
			// keys so schema.Validate reports the canonical missing_required
			// violation — aggregated with every other failure in
			// validateBeforeCreate — instead of an empty-string length or
			// null type violation.
			for _, k := range []string{"goal", "scope", "kind"} {
				if fv, ok := fm[k]; ok && (fv == nil || fv == "") {
					delete(fm, k)
				}
			}

			// The topic-ordinal types mint a fresh ordinal every call, so their
			// path can never collide with an existing file — no drift check.
			if !isTopicOrdinalType(t) {
				if existing, err := core.LoadArtifact(path); err == nil {
					if flagUpdate && !userAuthoredBody {
						// No body flag: the template scaffold is not a caller
						// intent, so it must neither count as drift nor replace
						// the authored body.
						body = existing.Body
					}
					drift := createDrift(t, fm, existing.FrontMatter, body, existing.Body)
					if drift == "" {
						return emitCreateResult(cmd, flagJSON, id, path, statusAlreadyExists, nil, nil, nil, snapshotResult{})
					}
					if !flagUpdate {
						return formatDriftError(cmd, id, drift, fm, existing.FrontMatter, body, existing.Body)
					}
					// --update path: keep every field the caller did not pass,
					// then re-validate the merged fm + body before overwriting.
					fm, changed := mergeUpdate(cmd, existing.FrontMatter, fm)
					if !sameBody(body, existing.Body) {
						changed = append(changed, "body")
					}
					findings, err := validateBeforeCreate(cmd, v, t, path, fm, body, userAuthoredBody, flagAllowNewFacet, flagJSON, preValidationErrors...)
					if err != nil {
						return err
					}
					snap := snapshotArtifact(v.Root, path, string(t)+"."+id)
					originalBytes, rerr := os.ReadFile(path) //nolint:gosec // path is test-controlled or application-managed; not user input
					if rerr != nil {
						return fmt.Errorf("reading existing artifact for rollback: %w", rerr)
					}
					a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
					if err := a.Save(); err != nil {
						return fmt.Errorf("saving artifact: %w", err)
					}
					if err := indexAfterSave(v, a); err != nil {
						indexErr := fmt.Errorf("indexing %s: %w", id, err)
						if werr := os.WriteFile(path, originalBytes, 0o644); werr != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
							return errors.Join(indexErr, fmt.Errorf("rolling back %s to prior contents: %w", path, werr))
						}
						return indexErr
					}
					return emitCreateResult(cmd, flagJSON, id, path, statusUpdated, nil, findings, changed, snap)
				} else if !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("checking %s: %w", path, err)
				}
			}

			findings, err := validateBeforeCreate(cmd, v, t, path, fm, body, userAuthoredBody, flagAllowNewFacet, flagJSON, preValidationErrors...)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
			}

			a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}

			if err := indexAfterSave(v, a); err != nil {
				indexErr := fmt.Errorf("indexing %s: %w", id, err)
				if rerr := os.Remove(path); rerr != nil {
					return errors.Join(indexErr, fmt.Errorf("rolling back: removing %s: %w", path, rerr))
				}
				return indexErr
			}
			var warnings []string
			if !flagForceNew {
				warnings = findNearDuplicates(v, t, project, id)
			}
			return emitCreateResult(cmd, flagJSON, id, path, statusCreated, warnings, findings, nil, snapshotResult{})
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "artifact title (required)")
	cmd.Flags().StringVar(&flagDescription, "description", "", fmt.Sprintf("one-line summary (max %d chars); defaults to --title for issue and milestone when omitted", maxDescriptionChars))
	cmd.Flags().StringVar(&flagGoal, "goal", "", fmt.Sprintf("terminal predicate, one sentence (max %d chars, required for issue and milestone)", maxGoalChars))
	cmd.Flags().StringVar(&flagProject, "project", "", "project slug (overrides auto-detected; supported on: "+strings.Join(core.TypesSupportingProject(), ", ")+"; inbox aliases to --suggested-project)")
	cmd.Flags().StringVar(&flagTopic, "topic", "", "topic slug scoping the NNNN ordinal (required for decision and thread)")
	cmd.Flags().StringVar(&flagSuggestedType, "suggested-type", "", "suggested type (inbox only)")
	cmd.Flags().StringVar(&flagSuggestedProject, "suggested-project", "", "suggested project (inbox only)")
	cmd.Flags().StringVar(&flagSlug, "slug", "", "override the title-derived slug (must match ^[a-z0-9][a-z0-9-]*$)")
	cmd.Flags().StringVar(&flagBody, "body", "", "artifact body content (literal, or '-' to read stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read artifact body from <path>; mutually exclusive with --body and piped stdin")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&flagBreaking, "breaking", false, "sweep is breaking (required for sweep, must be explicit)")
	cmd.Flags().StringVar(&flagScope, "scope", "", "sweep scope (required for sweep)")
	cmd.Flags().StringVar(&flagSessionID, "session-id", "", "session UUID (required for session)")
	cmd.Flags().StringVar(&flagSource, "source", "claude-code", "session source ("+strings.Join(validSessionSources, "|")+")")
	cmd.Flags().StringVar(&flagStartedAt, "started-at", "", "RFC3339 session start time (defaults to now)")
	cmd.Flags().StringVar(&flagActiveThread, "active-thread", "", "active thread slug to record in related[]")
	cmd.Flags().BoolVar(&flagUpdate, "update", false, "on drift, rewrite the existing artifact: passed flags and body replace; status, related and other unpassed fields are kept; the prior file is snapshotted to vault git first")
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "comma-separated tag list (e.g. domain/dbt,activity/testing)")
	cmd.Flags().StringSliceVar(&flagAllowNewFacet, "allow-new-facet", nil, "facet to suppress novelty gate for (repeatable: domain|activity|pattern)")
	cmd.Flags().BoolVar(&flagForceNew, "force-new", false, "skip the near-duplicate similarity check")
	cmd.Flags().StringVar(&flagSeverity, "severity", "", "issue severity (low|medium|high|critical; issue only)")
	cmd.Flags().StringVar(&flagMilestone, "milestone", "", "milestone slug or wikilink to assign (issue only)")
	cmd.Flags().StringArrayVar(&flagAcceptance, "acceptance", nil, "acceptance criterion to add (repeatable; issue, milestone)")
	cmd.Flags().StringVar(&flagKind, "kind", "", "component design kind (registered label, required — register via `anvil component-design kinds add`) or milestone kind (scoped, the default)")
	cmd.Flags().BoolVar(&flagShowTemplate, "show-template", false, "print the required body skeleton + tag rules for <type> and exit (learning, issue, milestone, component-design, product-design, system-design)")
	cmd.Flags().BoolVar(&flagSkipVerifyPredicates, "skip-verify-predicates", false, skipVerifyPredicatesFlagUsage)

	return cmd
}
