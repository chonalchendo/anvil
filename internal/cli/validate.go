package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/glossary"
	"github.com/chonalchendo/anvil/internal/index"
	"github.com/chonalchendo/anvil/internal/schema"
)

// hasBlockingFailure reports whether failures carries any finding above
// SeverityWarning. A warning is emitted but must not fail the run — the
// sweep's grandfather tier for the milestone body check (validate.go).
func hasBlockingFailure(failures []*errfmt.ValidationError) bool {
	for _, f := range failures {
		if f.Severity != errfmt.SeverityWarning {
			return true
		}
	}
	return false
}

func newValidateCmd() *cobra.Command {
	var asJSON bool
	var verificationStdin bool
	cmd := &cobra.Command{
		Use:     "validate [path]",
		Short:   "Validate vault frontmatter against schemas",
		Args:    cobra.MaximumNArgs(1),
		Example: "  anvil validate\n  anvil validate --json\n  anvil validate /path/to/vault\n  anvil validate skill\n  echo 'cd $HOME/Development/anvil' | anvil validate --verification-stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			if verificationStdin {
				return runVerificationStdinLint(cmd, asJSON)
			}
			var root, singleFile string
			if len(args) == 1 {
				fi, err := os.Stat(args[0])
				if err != nil {
					return fmt.Errorf("stat %s: %w", args[0], err)
				}
				if fi.IsDir() {
					root = args[0]
				} else {
					vaultRoot, err := vaultRootFromArtifactPath(args[0])
					if err != nil {
						return err
					}
					root = vaultRoot
					singleFile = args[0]
				}
			} else {
				v, err := core.ResolveVault()
				if err != nil {
					return err
				}
				root = v.Root
			}

			g, err := glossary.Load(glossary.Path(root))
			if err != nil {
				return fmt.Errorf("loading glossary: %w", err)
			}
			var known map[string]struct{}
			if tags := g.Tags(); len(tags) > 0 {
				known = make(map[string]struct{}, len(tags))
				for _, tag := range tags {
					known[tag] = struct{}{}
				}
			}

			verbs := verbPathValidator(cmd.Root())
			vault := &core.Vault{Root: root}
			var failures []*errfmt.ValidationError
			if singleFile != "" {
				t, err := typeFromArtifactPath(singleFile)
				if err != nil {
					return err
				}
				_, fs := validateOne(t, singleFile, known, verbs, vault, false)
				failures = fs
			} else {
				// idPaths accumulates every path seen per index id to detect
				// cross-file collisions after per-file checks complete. Under
				// --project, it holds only scoped artifacts, so duplicate-id
				// detection narrows to the named project — a foreign-vs-scoped
				// collision is out of scope for a scoped run by construction.
				projectFilter := os.Getenv("ANVIL_PROJECT")
				idPaths := make(map[string][]string)
				for _, t := range core.AllTypes {
					paths, err := collectArtifactPaths(root, t)
					if err != nil {
						return err
					}
					for _, p := range paths {
						a, fs := validateOne(t, p, known, verbs, vault, true)
						if projectFilter != "" && !artifactInProject(a, p, t, projectFilter) {
							continue
						}
						failures = append(failures, fs...)
						if a == nil {
							continue // parse failures already reported above
						}
						// Reuse the index's canonical id derivation so validate
						// detects exactly the collisions the indexer would.
						row, rowErr := index.ArtifactRowFromFrontmatter(a.FrontMatter, p)
						if rowErr != nil {
							continue
						}
						idPaths[row.ID] = append(idPaths[row.ID], p)
					}
				}
				// Report each id that maps to more than one file. Sort ids so
				// duplicate_id findings are emitted in a stable order (map
				// iteration is otherwise non-deterministic).
				ids := make([]string, 0, len(idPaths))
				for id := range idPaths {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				for _, id := range ids {
					paths := idPaths[id]
					if len(paths) < 2 {
						continue
					}
					// Use the first colliding path as the ValidationError anchor;
					// all colliding paths appear in Expected so both are visible.
					failures = append(failures, errfmt.NewValidationError(
						errfmt.CodeDuplicateID, paths[0], "id",
						"duplicate id: "+id,
					).WithExpected(paths))
				}
			}

			if asJSON {
				if failures == nil {
					failures = []*errfmt.ValidationError{}
				}
				b, _ := json.Marshal(failures)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				printValidationErrors(cmd, failures)
			}

			if hasBlockingFailure(failures) {
				return ErrSchemaInvalid
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of structured errors")
	cmd.Flags().BoolVar(&verificationStdin, "verification-stdin", false, "lint a Verification block's bash script (read from stdin) for a hardcoded checkout path that would override worktree anchoring, and for a non-gating `!` assertion set -e would exempt — the rules `create issue` enforces; no vault lookup, ignores [path], honours --json")
	cmd.AddCommand(newValidateSkillCmd())
	return cmd
}

// artifactInProject reports whether an artifact belongs to the named project
// slug. It resolves the project from the frontmatter `project` field — the same
// signal list.go's matchesFilters uses — so scoping is robust to a misfiled
// artifact (filename slug != declared project), which is exactly the hygiene
// defect validate exists to surface; a path-based filter would let such a file
// slip the scope. When frontmatter is missing (parse failure) the function falls
// back to the filename prefix.
func artifactInProject(a *core.Artifact, path string, _ core.Type, slug string) bool {
	if a != nil {
		if p, ok := a.FrontMatter["project"].(string); ok && p != "" {
			return p == slug
		}
	}
	return strings.HasPrefix(filepath.Base(path), slug+".")
}

// verbPathValidator builds a core.VerbPathValidator backed by cobra's command
// tree, so the Verification verb-lint validates the full `anvil <verb>
// <subverb>...` path, not just the top-level token. Cobra's Find walks the
// tree and returns the deepest matched command plus the unconsumed args; a path
// is bogus when that command still has subcommands and the first unconsumed arg
// names none of them (e.g. `anvil project init` — `project` is real, `init` is
// not). A leaf command (`anvil create issue`) consumes its trailing args as
// flags/positionals, so those never count as subcommand candidates.
func verbPathValidator(root *cobra.Command) core.VerbPathValidator {
	return func(tokens []string) (string, bool) {
		if len(tokens) == 0 {
			return "", true
		}
		cmd, rest, _ := root.Find(tokens)
		if !cmd.HasSubCommands() {
			return "", true
		}
		if len(rest) > 0 {
			// tokens are positional-only, so rest[0] sits in subcommand position
			// yet Find did not descend into it: it names no registered subcommand.
			return strings.Trim(rest[0], "()\"';"), false
		}
		return "", true
	}
}

// validateOne runs schema and type-specific body checks on a single artifact
// file and returns the loaded artifact alongside any structured failures. A
// schema failure does not short-circuit: type-specific body checks still run,
// so one artifact reports every violation class in one pass (anvil.0218). The
// artifact is nil on a parse failure (the only failure that prevents loading);
// callers reuse it for cross-file id-collision detection without a second load.
// sweep is true only for the multi-file vault-wide walk (as opposed to a
// single-file `anvil validate <path>` or the create/promote gate), so a
// type-specific check can grandfather the back catalogue at warning severity
// there while still refusing outright everywhere else.
func validateOne(t core.Type, path string, knownTags map[string]struct{}, verbs core.VerbPathValidator, v *core.Vault, sweep bool) (*core.Artifact, []*errfmt.ValidationError) {
	a, err := core.LoadArtifact(path)
	if err != nil {
		return nil, []*errfmt.ValidationError{errfmt.NewValidationError(errfmt.CodeParseError, path, "", err.Error())}
	}
	var out []*errfmt.ValidationError
	if err := schema.Validate(string(t), a.FrontMatter); err != nil {
		out = append(out, schemaErrToValidationErrors(path, err)...)
	}

	// Dangling frontmatter wikilinks in declared link slots (e.g. `related:`)
	// must be refused at write time, not first surfaced by completion-time
	// hydrate (anvil.0225). Body wikilinks stay create-time-only for now — the
	// back catalogue carries ~80 files with dangling body edges, so folding
	// them into validate is an explicit non-goal until that repair lands.
	for _, link := range core.ResolveLinks(v, a.FrontMatter) {
		out = append(out, unresolvedLinkError(path, link))
	}

	if t == core.TypeLearning {
		// ValidateLearning covers both body-shape and glossary membership for
		// learnings; the generic drift check below skips learnings to avoid
		// double-reporting.
		for _, vErr := range core.ValidateLearning(a, knownTags) {
			out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
		}
		return a, out
	}

	if t == core.TypeIssue {
		for _, vErr := range core.ValidateIssue(a) {
			out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
		}
		goal, _ := a.FrontMatter["goal"].(string)
		title, _ := a.FrontMatter["title"].(string)
		for _, vErr := range core.ValidateIssueVerbs(a.Body, goal, title, verbs) {
			out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
		}
		// ValidateIssueCheckoutPaths is deliberately NOT wired here: the
		// checkout-path lint gates create/promote only. The ~219 issues
		// authored before the rule would fail vault-hygiene CI retroactively
		// if the vault-wide scan enforced it. Lint a single predicate with
		// `anvil validate --verification-stdin` instead.
	}
	// lead_sentence is skipped on the vault-wide sweep: the back catalogue
	// carries ~1200 pre-rule artifacts, and the rule is a create/promote-time
	// backstop (writing-issue/writing-milestone prescribe it in prose), not a
	// retroactive rewrite obligation (anvil.0274 non-goal). Single-file
	// `anvil validate <path>` still runs it — sweep is false there.
	if !sweep {
		out = append(out, leadSentenceFailures(t, a.Body, path)...)
	}

	if t == core.TypeMilestone {
		for _, vErr := range core.ValidateMilestone(a) {
			e := errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error())
			if sweep {
				e = e.WithSeverity(errfmt.SeverityWarning)
			}
			out = append(out, e)
		}
	}

	// Drift check: flag tags not present in the glossary. Skipped when the
	// glossary is empty so fresh vaults don't fail until any tags are defined.
	if knownTags != nil {
		raw, _ := a.FrontMatter["tags"].([]any)
		for _, item := range raw {
			tag, ok := item.(string)
			if !ok {
				continue
			}
			if _, _, valid := glossary.SplitTag(tag); !valid {
				// Malformed shape — schema layer surfaces these.
				continue
			}
			if _, defined := knownTags[tag]; !defined {
				out = append(out, errfmt.NewValidationError(errfmt.CodeUnknownGlossaryTag, path, "tags", tag).
					WithFix(fmt.Sprintf(`add it via "anvil tags add %s --desc \"...\""`, tag)))
			}
		}
	}

	return a, out
}

// unresolvedLinkError renders a dangling wikilink in the one unresolved_link
// shape shared by both write-time surfaces — `anvil validate`'s frontmatter
// walk and `anvil create`'s authored-body scan — so the same defect class
// reports the same code and fix across verbs (convention.cli-tooling). Every
// UnresolvedLink target contains a dot: both producers skip dot-less tokens.
func unresolvedLinkError(path string, link core.UnresolvedLink) *errfmt.ValidationError {
	e := errfmt.NewValidationError(errfmt.CodeUnresolvedLink, path, link.Field,
		fmt.Sprintf("unresolved wikilink [[%s]]", link.Target))
	prefix := link.Target[:strings.IndexByte(link.Target, '.')]
	if _, err := core.ParseType(prefix); err != nil {
		// Only the body scan emits unknown-prefix targets; the frontmatter walk
		// ignores them as non-vault references.
		return e.WithFix("use a known `<type>.<id>` target form or remove the wikilink")
	}
	return e.WithFix(fmt.Sprintf("fix the target id or remove the wikilink — `anvil list %s` shows valid ids", prefix))
}

// vaultRootFromArtifactPath resolves the vault root for an artifact file by
// matching the parent directory name against the known type-dir set.
func vaultRootFromArtifactPath(path string) (string, error) {
	parent := filepath.Dir(path)
	for _, t := range core.AllTypes {
		if filepath.Base(parent) == t.Dir() {
			return filepath.Dir(parent), nil
		}
	}
	return "", errfmt.NewNotInVault(path)
}

// typeFromArtifactPath infers the Type from the artifact's parent dir. Every
// type — designs included — owns a type-pure folder, so the dir maps to one type.
func typeFromArtifactPath(path string) (core.Type, error) {
	parent := filepath.Base(filepath.Dir(path))
	for _, t := range core.AllTypes {
		if t.Dir() == parent {
			return t, nil
		}
	}
	return "", errfmt.NewNotInVault(path)
}
