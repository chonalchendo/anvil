package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugPattern matches a valid pre-formed slug (lowercase, digits, hyphens;
// must start with a letter or digit). Same pattern the schemas enforce on
// `slug:` fields.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidateSlug reports whether s is a well-formed slug. Returns an error
// naming the offending rune and its byte index when invalid.
func ValidateSlug(s string) error {
	if s == "" {
		return fmt.Errorf("slug is empty")
	}
	if slugPattern.MatchString(s) {
		return nil
	}
	for i, r := range s {
		if i == 0 {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return fmt.Errorf("slug %q: first character %q is invalid; must be a-z or 0-9", s, r)
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return fmt.Errorf("slug %q: character %q at byte %d is invalid; allowed: a-z 0-9 -", s, r, i)
		}
	}
	return fmt.Errorf("slug %q: does not match pattern %s", s, slugPattern)
}

// Slugify lowercases s, transliterates via NFD + ASCII-filter, collapses
// non-alnum runs to "-", trims leading/trailing "-", and clips to 60 chars.
func Slugify(s string) string {
	decomposed := norm.NFD.String(s)
	var asciiBuf strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue // strip combining marks
		}
		if r > 127 {
			continue
		}
		asciiBuf.WriteRune(unicode.ToLower(r))
	}
	slug := nonAlnum.ReplaceAllString(asciiBuf.String(), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = strings.TrimRight(slug[:60], "-")
	}
	return slug
}

// slugifyIssue applies Slugify and then truncates the result to 40 chars,
// breaking at the last "-" so the slug doesn't end mid-word. Used only for
// numbered issue filenames where the ordinal already provides uniqueness.
func slugifyIssue(s string) string {
	slug := Slugify(s)
	const maxSlugLen = 40
	if len(slug) <= maxSlugLen {
		return slug
	}
	cut := slug[:maxSlugLen]
	if i := strings.LastIndexByte(cut, '-'); i > 0 {
		cut = cut[:i]
	}
	return cut
}

// numberedIssueRe matches <project>.NNNN.<slug>.md — used by ordinal scan.
var numberedIssueRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.([0-9]+)\.[a-z0-9][a-z0-9-]*\.md$`)

// nextIssueOrdinal scans the issues directory for files matching
// <project>.NNNN.<slug>.md and returns max(ordinal)+1 scoped to that project.
func nextIssueOrdinal(v *Vault, project string) (int, error) {
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("reading issues dir: %w", err)
	}
	prefix := project + "."
	highest := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		m := numberedIssueRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > highest {
			highest = n
		}
	}
	return highest + 1, nil
}

// AllocateIssueID allocates the next numbered issue ID for project using an
// atomic O_CREAT|O_EXCL probe on the target path, retrying on EEXIST. The
// file created by the probe is immediately removed; the caller writes the real
// content via the normal create path. Returns the ID string
// (<project>.NNNN.<slug>) and the resolved path.
//
// Slug is derived from title via slugifyIssue unless slugOverride is non-empty,
// in which case slugOverride (already validated) is used directly.
func AllocateIssueID(v *Vault, project, title, slugOverride string) (id, path string, err error) {
	slug := slugOverride
	if slug == "" {
		slug = slugifyIssue(title)
	} else if err := ValidateSlug(slug); err != nil {
		return "", "", err
	}
	if slug == "" {
		return "", "", fmt.Errorf("title required (produced empty slug)")
	}
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	// Idempotency (agent-cli-principles §6): a re-create with the same slug
	// resolves to the existing issue so the caller's drift check runs (no-op /
	// drift error / --update) rather than minting a duplicate under a fresh
	// ordinal. Only a genuinely-new slug allocates a new ordinal below.
	if existingID, existingPath, found := findIssueBySlug(v, project, slug); found {
		return existingID, existingPath, nil
	}
	for attempt := 0; attempt < 20; attempt++ {
		ordinal, err := nextIssueOrdinal(v, project)
		if err != nil {
			return "", "", err
		}
		candidate := fmt.Sprintf("%s.%04d.%s", project, ordinal, slug)
		candidatePath := filepath.Join(dir, candidate+".md")
		f, err := os.OpenFile(candidatePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		if err == nil {
			// Probe succeeded; remove placeholder so the real content write can proceed.
			_ = f.Close()
			if rerr := os.Remove(candidatePath); rerr != nil {
				return "", "", fmt.Errorf("removing probe file %s: %w", candidatePath, rerr)
			}
			return candidate, candidatePath, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", fmt.Errorf("probing %s: %w", candidatePath, err)
		}
		// EEXIST: another writer landed between the scan and the probe; retry.
	}
	return "", "", fmt.Errorf("unable to allocate numbered issue ID after 20 attempts")
}

// findIssueBySlug returns the ID (without .md) and path of an existing
// <project>.NNNN.<slug>.md whose slug matches exactly, scoped to project.
// Returns ("", "", false) when none exists. Slug is unique per project under
// the numbered scheme, so at most one file matches.
func findIssueBySlug(v *Vault, project, slug string) (id, path string, found bool) {
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", false
	}
	prefix := project + "."
	suffix := "." + slug + ".md"
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		ordinal := name[len(prefix) : len(name)-len(suffix)]
		if ordinal != "" && ordinalOnlyRe.MatchString(ordinal) {
			return strings.TrimSuffix(name, ".md"), filepath.Join(dir, name), true
		}
	}
	return "", "", false
}

// ordinalOnlyRe matches a string that is only digits — a bare ordinal like "0042".
var ordinalOnlyRe = regexp.MustCompile(`^[0-9]+$`)

// projectQualifiedOrdinalRe matches <project>.NNNN — e.g. "anvil.0018".
// Capture groups: 1=project, 2=ordinal.
var projectQualifiedOrdinalRe = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)\.([0-9]+)$`)

// IsOrdinalOnly reports whether s is a bare issue ordinal (all digits, no dots).
func IsOrdinalOnly(s string) bool { return ordinalOnlyRe.MatchString(s) }

// ParseProjectQualifiedOrdinal parses a "<project>.NNNN" string and returns
// the project slug and ordinal digits. Returns ("", "", false) for any other form.
func ParseProjectQualifiedOrdinal(s string) (project, ordinal string, ok bool) {
	m := projectQualifiedOrdinalRe.FindStringSubmatch(s)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// ResolveIssueOrdinal scans the issues directory for a file matching
// <project>.NNNN.<slug>.md where NNNN == ordinal, and returns the full ID
// (without the .md extension). Returns ("", false) when no match is found.
// project must already be resolved by the caller.
func ResolveIssueOrdinal(v *Vault, project, ordinal string) (string, bool) {
	if !ordinalOnlyRe.MatchString(ordinal) {
		return "", false
	}
	n, err := strconv.Atoi(ordinal)
	if err != nil {
		return "", false
	}
	prefix := fmt.Sprintf("%s.%04d.", project, n)
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".md") {
			return strings.TrimSuffix(name, ".md"), true
		}
	}
	return "", false
}

// ResolveIssueArg canonicalises a user-supplied issue argument to a full issue
// ID so read (show/list) and write (set/transition) paths accept identical
// forms. It handles, in order:
//   - a qualified "issue.<project>.<slug>" wikilink — strips the "issue."
//     prefix (only when the remainder still contains "." so the bare
//     "issue.<project>" singleton form is left intact);
//   - a project-qualified ordinal "<project>.NNNN" — resolved against the
//     issues directory;
//   - a bare ordinal "NNNN" — project taken from cwd context.
//
// Any argument that is none of these (an already-full ID, an unresolvable
// ordinal) is returned unchanged so the caller's path-load surfaces the
// not-found error.
func ResolveIssueArg(v *Vault, arg string) string {
	const prefix = string(TypeIssue) + "."
	if rest, found := strings.CutPrefix(arg, prefix); found && strings.Contains(rest, ".") {
		arg = rest
	}
	if proj, ord, ok := ParseProjectQualifiedOrdinal(arg); ok {
		if resolved, ok := ResolveIssueOrdinal(v, proj, ord); ok {
			return resolved
		}
		return arg
	}
	if IsOrdinalOnly(arg) {
		if p, err := ResolveProject(); err == nil {
			if resolved, ok := ResolveIssueOrdinal(v, p.Slug, arg); ok {
				return resolved
			}
		}
	}
	return arg
}

// IDInputs carries optional fields used by some artifact types.
type IDInputs struct {
	Title   string // required — slug source when Slug is empty
	Project string // required for issue/plan/milestone
	Topic   string // required for decision
	Slug    string // optional — when set, overrides title-derived slug
}

// DeterministicID returns the slug-keyed ID a given type would receive
// before any collision-suffix is applied. Returns an error for decisions
// (which require a vault scan to allocate an ordinal).
func DeterministicID(t Type, in IDInputs) (string, error) {
	slug := in.Slug
	if slug == "" {
		slug = Slugify(in.Title)
	} else if err := ValidateSlug(slug); err != nil {
		return "", err
	}
	if slug == "" {
		return "", fmt.Errorf("title required (produced empty slug)")
	}
	switch t {
	case TypeInbox:
		date := time.Now().UTC().Format("2006-01-02")
		return fmt.Sprintf("%s-%s", date, slug), nil
	case TypeIssue, TypePlan, TypeMilestone, TypeContract, TypeSystemDesign:
		if in.Project == "" {
			return "", fmt.Errorf("project required for %s", t)
		}
		return fmt.Sprintf("%s.%s", in.Project, slug), nil
	case TypeThread, TypeLearning, TypeSweep:
		return slug, nil
	case TypeDecision:
		return "", fmt.Errorf("decision IDs are not deterministic (ordinal-scoped)")
	}
	return "", fmt.Errorf("unknown type %q", t)
}

// NextID returns the next available ID for type t under v.
// Decisions can't delegate to DeterministicID because the ordinal must be
// allocated by scanning the vault; for all other types DeterministicID is
// the slug-keyed base and uniqueID handles collision suffixes.
func NextID(v *Vault, t Type, in IDInputs) (string, error) {
	if t == TypeDecision {
		if in.Topic == "" {
			return "", fmt.Errorf("topic required for decision")
		}
		slug := in.Slug
		if slug == "" {
			slug = Slugify(in.Title)
		} else if err := ValidateSlug(slug); err != nil {
			return "", err
		}
		if slug == "" {
			return "", fmt.Errorf("title required (produced empty slug)")
		}
		n, err := nextDecisionOrdinal(v, in.Topic)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%04d-%s", in.Topic, n, slug), nil
	}
	base, err := DeterministicID(t, in)
	if err != nil {
		return "", err
	}
	return uniqueID(v, t, base)
}

// uniqueID returns base, or base-2, base-3, ... whichever does not yet exist on disk.
func uniqueID(v *Vault, t Type, base string) (string, error) {
	// Path resolves every AllocatesID form: <Dir>/<id>.md for flat types,
	// the nested <project>/system-design.<shard>.md for system-design shards.
	pathFor := func(id string) string { return t.Path(v.Root, "", id) }
	if !fileExists(pathFor(base)) {
		return base, nil
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !fileExists(pathFor(candidate)) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to allocate unique ID for %s after 1000 attempts", base)
}

// nextDecisionOrdinal scans the decisions directory for files matching
// <topic>.NNNN-*.md and returns the next ordinal scoped to that topic.
func nextDecisionOrdinal(v *Vault, topic string) (int, error) {
	dir := filepath.Join(v.Root, TypeDecision.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("reading decisions dir: %w", err)
	}
	prefix := topic + "."
	highest := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".md") {
			continue
		}
		// Format: <topic>.NNNN-<slug>.md
		rest := strings.TrimSuffix(name[len(prefix):], ".md")
		dash := strings.IndexByte(rest, '-')
		if dash < 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:dash])
		if err != nil {
			continue
		}
		if n > highest {
			highest = n
		}
	}
	return highest + 1, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
