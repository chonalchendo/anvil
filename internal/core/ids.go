package core

import (
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

// IDInputs carries optional fields used by some artifact types.
type IDInputs struct {
	Title   string // required — slug source when Slug is empty
	Project string // required for issue/plan/milestone
	Topic   string // required for decision and thread
	Slug    string // optional — when set, overrides title-derived slug
}

// DeterministicID returns the slug-keyed ID a given type would receive
// before any collision-suffix is applied. Returns an error for the
// topic-ordinal types (decision, thread), which require a vault scan to
// allocate an ordinal.
//
// Issue, milestone, contract, plan and convention ids keep their `<type>.`
// prefix, so the id, the on-disk basename and the `[[type.id]]` wikilink are
// one string. Design types (product-design, system-design) key on a bare
// project slug instead — the index (core.IndexKey) disambiguates a bare id
// shared across the two design types, so the prefix is no longer needed for
// uniqueness. Design types are handled before the slug validation because
// they don't require a title.
func DeterministicID(t Type, in IDInputs) (string, error) {
	switch t {
	case TypeProductDesign:
		// ID is always <project> — no slug component.
		if in.Project == "" {
			return "", fmt.Errorf("project required for %s", t)
		}
		return in.Project, nil
	case TypeSystemDesign:
		// ID is <project> for the singleton, or <project>.<slug> for a named
		// shard (explicit --slug only; title does not derive a shard).
		if in.Project == "" {
			return "", fmt.Errorf("project required for %s", t)
		}
		if in.Slug == "" {
			return in.Project, nil
		}
		if err := ValidateSlug(in.Slug); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s", in.Project, in.Slug), nil
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
	switch t {
	case TypeInbox:
		date := time.Now().UTC().Format("2006-01-02")
		return fmt.Sprintf("%s-%s", date, slug), nil
	case TypeIssue, TypePlan, TypeMilestone, TypeContract:
		if in.Project == "" {
			return "", fmt.Errorf("project required for %s", t)
		}
		return fmt.Sprintf("%s.%s.%s", t, in.Project, slug), nil
	case TypeLearning, TypeSweep:
		return slug, nil
	case TypeConvention:
		// Conventions are project-agnostic, slug-keyed, and keep the type prefix
		// in the id (convention.<slug>): the bare slug ("python") would collide
		// with same-named artifacts of other types, and (unlike the design
		// types) conventions have no folder-scoped project to key the index on
		// instead.
		return fmt.Sprintf("%s.%s", t, slug), nil
	case TypeDecision, TypeThread:
		return "", fmt.Errorf("%s IDs are not deterministic (topic-ordinal scoped)", t)
	}
	return "", fmt.Errorf("unknown type %q", t)
}

// IndexKey maps an id to the string vault.db's artifacts/links/tags/fts
// tables key on. Design types (product-design, system-design) mint a bare,
// folder-scoped CanonicalID (see DeterministicID), so two designs sharing a
// project would collide on a bare index key — IndexKey type-qualifies exactly
// those two types (equivalent to WikilinkTarget), while every other type's
// index key stays identical to its CanonicalID.
func IndexKey(t Type, id string) string {
	switch t {
	case TypeProductDesign, TypeSystemDesign:
		return WikilinkTarget(t, id)
	}
	return CanonicalID(t, id)
}

// NextID returns the next available ID for type t under v.
// The topic-ordinal types (decision, thread) can't delegate to DeterministicID
// because the ordinal must be allocated by scanning the vault; for all other
// types DeterministicID is the slug-keyed base and uniqueID handles collision
// suffixes.
func NextID(v *Vault, t Type, in IDInputs) (string, error) {
	if t == TypeDecision || t == TypeThread {
		if in.Topic == "" {
			return "", fmt.Errorf("topic required for %s", t)
		}
		// The topic must be slug-shaped because SplitTopicOrdinal keys on the
		// first dot: a dotted topic ("v0.2") makes nextTopicOrdinal blind to its
		// own files, so every create mints 0001 and a repeated title silently
		// overwrites the prior artifact.
		if err := ValidateSlug(in.Topic); err != nil {
			return "", fmt.Errorf("topic must be a slug (lowercase, hyphenated, no dots): %w", err)
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
		n, err := nextTopicOrdinal(v, t, in.Topic)
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

// uniqueID returns base, or base-2, base-3, ... whichever does not yet exist as <dir>/<id>.md.
func uniqueID(v *Vault, t Type, base string) (string, error) {
	dir := filepath.Join(v.Root, t.Dir())
	if !fileExists(filepath.Join(dir, base+".md")) {
		return base, nil
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !fileExists(filepath.Join(dir, candidate+".md")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to allocate unique ID for %s after 1000 attempts", base)
}

// SplitTopicOrdinal splits a topic-ordinal id (`<topic>.<NNNN>-<slug>`, as
// decision and thread mint) into its parts. ok is false for any other shape —
// including the bare-slug thread back catalogue, which keeps resolving as-is.
func SplitTopicOrdinal(id string) (topic string, ordinal int, slug string, ok bool) {
	dot := strings.IndexByte(id, '.')
	if dot < 0 {
		return "", 0, "", false
	}
	rest := id[dot+1:]
	dash := strings.IndexByte(rest, '-')
	if dash <= 0 {
		return "", 0, "", false
	}
	n, err := strconv.Atoi(rest[:dash])
	if err != nil {
		return "", 0, "", false
	}
	return id[:dot], n, rest[dash+1:], true
}

// nextTopicOrdinal scans t's directory for files matching <topic>.NNNN-*.md
// and returns the next ordinal scoped to that topic.
func nextTopicOrdinal(v *Vault, t Type, topic string) (int, error) {
	dir := filepath.Join(v.Root, t.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("reading %s dir: %w", t, err)
	}
	highest := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		tp, n, _, ok := SplitTopicOrdinal(strings.TrimSuffix(name, ".md"))
		if !ok || tp != topic {
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
