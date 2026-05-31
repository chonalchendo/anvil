package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// maxDescriptionChars mirrors the `maxLength: 120` cap in every spine-type
// schema (issue, plan, milestone, decision, sweep, product-design,
// system-design). Pre-flighted here so the CLI rejects oversize descriptions
// before any template rendering or facet walk, with a single focused error.
const maxDescriptionChars = 120

// maxGoalChars bounds an issue's `goal:` — the one-sentence terminal predicate.
// Same cap as description: it is a spine field, not a place for prose.
const maxGoalChars = 120

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
		flagGoal             string
		flagProject          string
		flagTopic            string
		flagSuggestedType    string
		flagSuggestedProject string
		flagSlug             string
		flagJSON             bool
		flagIssue            string
		flagBody             string
		flagBodyFile         string
		flagFrom             string
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
		Args:  namedArgs("anvil create <type>", []string{"<type>"}, 1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return err
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
						flagSuggestedProject = flagProject
					}
					flagProject = ""
				} else {
					return printAndReturn(cmd, errfmt.NewUnsupportedFlagForType(
						"project", string(t), core.TypesSupportingProject(),
						"this type is deliberately cross-project; omit --project",
					))
				}
			}

			// --from ingests a complete authored artifact (frontmatter + body) so
			// callers can avoid the create-stub-then-edit round-trip when the
			// frontmatter carries rich content (e.g. plan tasks). CLI flags still
			// own identity (id, created, slug) and override matching file fields
			// when explicitly set; gaps fall through to the file's values.
			var inputFM map[string]any
			var inputBody string
			if flagFrom != "" {
				if t != core.TypePlan {
					return fmt.Errorf("--from is supported for plan only")
				}
				if flagBody != "" {
					return errors.New("--from and --body are mutually exclusive")
				}
				if flagBodyFile != "" {
					return errors.New("--from and --body-file are mutually exclusive")
				}
				var content []byte
				if flagFrom == "-" {
					b, err := io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return fmt.Errorf("read stdin: %w", err)
					}
					content = b
				} else {
					b, err := os.ReadFile(flagFrom) //nolint:gosec // G304: flagFrom is the --from flag; reading a path the invoking user supplied is the command's purpose
					if err != nil {
						return fmt.Errorf("read %s: %w", flagFrom, err)
					}
					content = b
				}
				a, err := core.ParseArtifact(content)
				if err != nil {
					return fmt.Errorf("parse --from content: %w", err)
				}
				// Reject non-plan inputs early so fields like `severity` from an
				// issue can't leak through the later merge into a plan artifact.
				if ty, ok := a.FrontMatter["type"].(string); ok && ty != "" && ty != "plan" {
					return fmt.Errorf("--from input has type %q; expected plan", ty)
				}
				inputFM, inputBody = a.FrontMatter, a.Body

				// CLI-set values win; file fills gaps for identity fields the
				// existing per-type required-flag checks already cover.
				if !cmd.Flags().Changed("title") {
					if s, ok := inputFM["title"].(string); ok {
						flagTitle = s
					}
				}
				if !cmd.Flags().Changed("description") {
					if s, ok := inputFM["description"].(string); ok {
						flagDescription = s
					}
				}
				if !cmd.Flags().Changed("project") {
					if s, ok := inputFM["project"].(string); ok {
						flagProject = s
					}
				}
				if !cmd.Flags().Changed("issue") {
					if s, ok := inputFM["issue"].(string); ok {
						flagIssue = s
					}
				}
				if !cmd.Flags().Changed("tags") {
					if vs, ok := inputFM["tags"].([]any); ok {
						for _, x := range vs {
							if s, ok := x.(string); ok {
								flagTags = append(flagTags, s)
							}
						}
					}
				}
			}

			if t != core.TypeSession && flagTitle == "" {
				return fmt.Errorf("--title is required for %s", t)
			}

			// Capped-field checks run before vault/project resolution so cap
			// feedback fast-fails for every type, in or out of a vault. The
			// description cap applies to all spine types; the issue path also
			// collects its goal-length overage so the author sees every
			// violation in one rejection rather than one per resubmit.
			if t != core.TypeSession {
				var capErrs []error
				if n := utf8.RuneCountInString(flagDescription); n > maxDescriptionChars {
					capErrs = append(capErrs, fmt.Errorf(
						"--description too long: %d chars (max %d); description is spine index/preview text, not docs — re-summarise to fit the cap rather than raise it",
						n, maxDescriptionChars,
					))
				}
				if (t == core.TypeIssue || t == core.TypeMilestone) && strings.TrimSpace(flagGoal) != "" {
					if n := utf8.RuneCountInString(flagGoal); n > maxGoalChars {
						capErrs = append(capErrs, fmt.Errorf(
							"--goal too long: %d chars (max %d); goal is a one-sentence predicate, not docs — tighten it",
							n, maxGoalChars,
						))
					}
				}
				if err := errors.Join(capErrs...); err != nil {
					return err
				}
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

			// Per-type required-flag checks. Capped-field overages were already
			// reported above (before vault resolution); the issue goal cap is
			// surfaced there too when goal is present, so by here a missing goal
			// is the only remaining issue-specific failure.
			switch t {
			case core.TypeIssue:
				if strings.TrimSpace(flagGoal) == "" {
					return fmt.Errorf("--goal is required for issue: a one-sentence terminal predicate (what 'done' means)")
				}
			case core.TypeMilestone:
				if strings.TrimSpace(flagGoal) == "" {
					return fmt.Errorf("--goal is required for milestone: a one-sentence terminal predicate (what 'done' means)")
				}
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

			// Derive description from title when omitted for spine types that
			// require it, mirroring promote's single-step stub behaviour (see
			// promote.go: Description: title). The author refines via anvil set.
			if flagDescription == "" && (t == core.TypeIssue || t == core.TypeMilestone) {
				flagDescription = flagTitle
			}

			// Plan default slug derives from the linked issue's slug, not the
			// plan's own title. Same-slug pairing makes drift between linked
			// artifacts a typo, not the default. --slug still wins; pass it to
			// override (e.g. issue→plans fan-out where each plan needs its own
			// slug).
			slugDefault := flagSlug
			if slugDefault == "" && t == core.TypePlan && flagIssue != "" {
				if s, ok := slugFromIssueLink(flagIssue, project); ok {
					slugDefault = s
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
					Slug:    slugDefault,
				})
				if err != nil {
					return invalidSlugError(slugDefault, err)
				}
				id = allocated
			// Issues use a per-project atomic counter: <project>.NNNN.<slug>.md.
			case t == core.TypeIssue:
				allocated, allocPath, err := core.AllocateIssueID(v, project, flagTitle, slugDefault)
				if err != nil {
					return invalidSlugError(slugDefault, err)
				}
				id = allocated
				path = allocPath
			case t.AllocatesID():
				base, err := core.DeterministicID(t, core.IDInputs{
					Title:   flagTitle,
					Project: project,
					Slug:    slugDefault,
				})
				if err != nil {
					return invalidSlugError(slugDefault, err)
				}
				id = base
			default:
				id = string(t)
			}
			if path == "" {
				path = t.Path(v.Root, project, id)
			}

			var body string
			// userAuthoredBody flags whether the agent supplied body content
			// (via --body, --body-file, --body -, piped stdin, or --from). When
			// true, validation runs body checks (section shape + wikilink
			// resolution). When false, the body is a CLI-generated stub and only
			// the frontmatter is validated.
			var userAuthoredBody bool
			if inputFM != nil {
				body = inputBody
				userAuthoredBody = true
			} else {
				body, err = readBody(cmd, flagBody, flagBodyFile)
				if err != nil {
					return err
				}
				userAuthoredBody = cmd.Flags().Changed("body") || cmd.Flags().Changed("body-file") || body != ""
				if body == "" && !cmd.Flags().Changed("body") && !cmd.Flags().Changed("body-file") {
					var sections []string
					switch t {
					case core.TypeLearning:
						sections = core.RequiredLearningSections
					case core.TypeIssue:
						sections = core.RequiredIssueSections
					}
					var sb strings.Builder
					for _, h := range sections {
						sb.WriteString("\n")
						sb.WriteString(h)
						sb.WriteString("\n")
					}
					body = sb.String()
				}
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

			// Overlay --from frontmatter onto the template-rendered fm. Identity
			// fields stay CLI/template-owned; CLI-controllable fields were already
			// folded in above; everything else (tasks, verification, plan-specific
			// authoring) comes from the file.
			for k, v := range inputFM {
				switch k {
				case "type", "id", "slug", "created", "updated", "status", "plan_version",
					"title", "description", "project", "issue", "tags":
					continue
				}
				fm[k] = v
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
					// --update path: preserve `created`, then re-validate the new
					// fm + body in one pass before overwriting.
					if c, ok := existing.FrontMatter["created"]; ok {
						fm["created"] = c
					}
					if err := validateBeforeCreate(cmd, v, t, path, fm, body, userAuthoredBody, flagAllowNewFacet, flagJSON); err != nil {
						return err
					}
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
					return emitCreateResult(cmd, flagJSON, id, path, statusUpdated, nil)
				} else if !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("checking %s: %w", path, err)
				}
			}

			if err := validateBeforeCreate(cmd, v, t, path, fm, body, userAuthoredBody, flagAllowNewFacet, flagJSON); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
			}

			// The T1 placeholder is a bootstrap convenience for the empty-handed
			// path. With --from the user is explicitly authoring the artifact,
			// so respect their (possibly empty) body rather than overwriting it.
			if t == core.TypePlan && body == "" && flagFrom == "" {
				body = "\n## Task: T1\n\n" + strings.Repeat(
					"Replace this with the RED test, expected failure, GREEN sketch, verify+commit. ", 4) + "\n"
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
			return emitCreateResult(cmd, flagJSON, id, path, statusCreated, warnings)
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "artifact title (required)")
	cmd.Flags().StringVar(&flagDescription, "description", "", fmt.Sprintf("one-line summary (max %d chars); defaults to --title for issue and milestone when omitted", maxDescriptionChars))
	cmd.Flags().StringVar(&flagGoal, "goal", "", fmt.Sprintf("terminal predicate, one sentence (max %d chars, required for issue and milestone)", maxGoalChars))
	cmd.Flags().StringVar(&flagProject, "project", "", "project slug (overrides auto-detected; supported on: "+strings.Join(core.TypesSupportingProject(), ", ")+"; inbox aliases to --suggested-project)")
	cmd.Flags().StringVar(&flagTopic, "topic", "", "decision topic slug (required for decision)")
	cmd.Flags().StringVar(&flagSuggestedType, "suggested-type", "", "suggested type (inbox only)")
	cmd.Flags().StringVar(&flagSuggestedProject, "suggested-project", "", "suggested project (inbox only)")
	cmd.Flags().StringVar(&flagSlug, "slug", "", "override the title-derived slug (must match ^[a-z0-9][a-z0-9-]*$)")
	cmd.Flags().StringVar(&flagIssue, "issue", "", "issue wikilink (required for plan)")
	cmd.Flags().StringVar(&flagBody, "body", "", "artifact body content (literal, or '-' to read stdin)")
	cmd.Flags().StringVar(&flagBodyFile, "body-file", "", "read artifact body from <path>; mutually exclusive with --body and piped stdin")
	cmd.Flags().StringVar(&flagFrom, "from", "", "read a complete artifact (frontmatter + body) from <path> or - for stdin; plan only")
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
