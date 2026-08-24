package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

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

// AmbiguousOrdinalError is returned when ordinal shorthand matches more than
// one issue file — a synced clone re-minted an ordinal the origin had already
// used. Only the full id can name one of them.
type AmbiguousOrdinalError struct {
	Ordinal    string   // <project>.NNNN
	Candidates []string // canonical ids, sorted
}

func (e *AmbiguousOrdinalError) Error() string {
	return fmt.Sprintf("ambiguous ordinal %s resolves to %d issues: %s — pass the full id, or `anvil renumber issue <id>` to move one onto a free ordinal",
		e.Ordinal, len(e.Candidates), strings.Join(e.Candidates, ", "))
}

// resolveIssueOrdinalAll returns every canonical issue id carrying
// <project>.NNNN, in directory order. Empty when none match.
func resolveIssueOrdinalAll(v *Vault, project, ordinal string) []string {
	if !ordinalOnlyRe.MatchString(ordinal) {
		return nil
	}
	n, err := strconv.Atoi(ordinal)
	if err != nil {
		return nil
	}
	prefix := fmt.Sprintf("%s.%04d.", project, n)
	entries, err := os.ReadDir(filepath.Join(v.Root, TypeIssue.Dir()))
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		name := BareID(TypeIssue, e.Name())
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".md") {
			ids = append(ids, CanonicalID(TypeIssue, strings.TrimSuffix(name, ".md")))
		}
	}
	return ids
}

// ResolveIssueOrdinal resolves <project>.NNNN to the one full issue id
// carrying it. ok is false when none matches; err is an *AmbiguousOrdinalError
// when more than one does — the shorthand must refuse rather than pick the
// file that happens to sort first.
func ResolveIssueOrdinal(v *Vault, project, ordinal string) (id string, ok bool, err error) {
	ids := resolveIssueOrdinalAll(v, project, ordinal)
	switch len(ids) {
	case 0:
		return "", false, nil
	case 1:
		return ids[0], true, nil
	}
	sort.Strings(ids)
	n, _ := strconv.Atoi(ordinal)
	return "", false, &AmbiguousOrdinalError{Ordinal: fmt.Sprintf("%s.%04d", project, n), Candidates: ids}
}

// ResolveIssueArg canonicalises a user-supplied issue argument to the full
// canonical issue ID (issue.<project>.NNNN.<slug>) so read (show/list) and
// write (set/transition) paths accept identical forms. It handles, in order:
//   - a project-qualified ordinal "<project>.NNNN" — resolved against the
//     issues directory;
//   - a bare ordinal "NNNN" — project taken from cwd context;
//   - anything else — normalised through CanonicalID, so an id given with or
//     without its "issue." prefix lands on the same shape.
//
// An unresolvable ordinal is returned in canonical shape so the caller's
// path-load surfaces the not-found error against the id issues mint under.
// An ordinal carried by more than one file returns *AmbiguousOrdinalError.
func ResolveIssueArg(v *Vault, arg string) (string, error) {
	bare := BareID(TypeIssue, arg)
	if proj, ord, ok := ParseProjectQualifiedOrdinal(bare); ok {
		resolved, ok, err := ResolveIssueOrdinal(v, proj, ord)
		if err != nil {
			return "", err
		}
		if ok {
			return resolved, nil
		}
		return CanonicalID(TypeIssue, bare), nil
	}
	if IsOrdinalOnly(bare) {
		if p, err := ResolveProject(); err == nil {
			resolved, ok, err := ResolveIssueOrdinal(v, p.Slug, bare)
			if err != nil {
				return "", err
			}
			if ok {
				return resolved, nil
			}
		}
		return CanonicalID(TypeIssue, bare), nil
	}
	return CanonicalID(TypeIssue, arg), nil
}

// numberedIssueRe matches <project>.NNNN.<slug>.md — used by ordinal scan.
var numberedIssueRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.([0-9]+)\.[a-z0-9][a-z0-9-]*\.md$`)

// ordinalReservationsDir is the vault-scoped directory holding one marker file
// per in-flight ordinal allocation. Vault-scoped because every concurrently
// creating process on the host shares the vault — a per-process or temp-scoped
// location would serialise nothing.
func ordinalReservationsDir(v *Vault) string {
	return filepath.Join(v.Root, ".anvil", "ordinals")
}

// nextIssueOrdinal returns max(ordinal)+1 for project, taken over both the
// issue files on disk and the live reservations — an ordinal reserved by a
// create still writing its file is already spoken for.
func nextIssueOrdinal(v *Vault, project string) (int, error) {
	highest := 0
	take := func(n int) {
		if n > highest {
			highest = n
		}
	}

	entries, err := os.ReadDir(filepath.Join(v.Root, TypeIssue.Dir()))
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("reading issues dir: %w", err)
	}
	prefix := project + "."
	for _, e := range entries {
		name := BareID(TypeIssue, e.Name())
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		m := numberedIssueRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			take(n)
		}
	}

	reserved, err := os.ReadDir(ordinalReservationsDir(v))
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("reading ordinal reservations: %w", err)
	}
	for _, e := range reserved {
		proj, ord, ok := ParseProjectQualifiedOrdinal(e.Name())
		if !ok || proj != project {
			continue
		}
		if n, err := strconv.Atoi(ord); err == nil {
			take(n)
		}
	}
	return highest + 1, nil
}

// AllocateIssueID allocates the next numbered issue ID for project by claiming
// an ordinal-keyed reservation marker with an atomic O_CREAT|O_EXCL create,
// retrying on EEXIST. Returns the ID string (<project>.NNNN.<slug>), the
// resolved path, and a release func the caller must call once the artifact is
// written (or the create has failed) to free the reservation.
//
// The marker is keyed on the ordinal alone, not the target filename: two
// concurrent creates pick different slugs, so a probe on the slug-bearing path
// never collides and both would mint the same ordinal. It is held for the whole
// create — not removed on the spot — because the caller's write lands long after
// allocation (body validation runs the issue's verification blocks first), and
// an ordinal freed inside that window is an ordinal a sibling session re-mints.
// A process killed mid-create leaks a marker, which costs one skipped ordinal.
//
// Slug is derived from title via slugifyIssue unless slugOverride is non-empty,
// in which case slugOverride (already validated) is used directly.
func AllocateIssueID(v *Vault, project, title, slugOverride string) (id, path string, release func(), err error) {
	noop := func() {}
	slug := slugOverride
	if slug == "" {
		slug = slugifyIssue(title)
	} else if err := ValidateSlug(slug); err != nil {
		return "", "", noop, err
	}
	if slug == "" {
		return "", "", noop, fmt.Errorf("title required (produced empty slug)")
	}
	// Idempotency (agent-cli-principles §6): a re-create with the same slug
	// resolves to the existing issue so the caller's drift check runs (no-op /
	// drift error / --update) rather than minting a duplicate under a fresh
	// ordinal. Only a genuinely-new slug allocates a new ordinal below.
	if existingID, existingPath, found := findIssueBySlug(v, project, slug); found {
		return existingID, existingPath, noop, nil
	}
	return ReserveIssueOrdinal(v, project, slug, 0)
}

// ReserveIssueOrdinal claims an ordinal for project via the reservation
// marker and returns the id/path it mints for slug. want == 0 takes the next
// free ordinal; want > 0 claims exactly that one and fails if a file or a live
// reservation already holds it. The release func frees the marker.
func ReserveIssueOrdinal(v *Vault, project, slug string, want int) (id, path string, release func(), err error) {
	noop := func() {}
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	reserveDir := ordinalReservationsDir(v)
	if err := os.MkdirAll(reserveDir, 0o750); err != nil {
		return "", "", noop, fmt.Errorf("creating ordinal reservations dir: %w", err)
	}
	claim := func(ordinal int) (id, path string, release func(), err error) {
		marker := filepath.Join(reserveDir, fmt.Sprintf("%s.%04d", project, ordinal))
		f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // marker is a zero-byte lock; 0644 keeps it removable by any session sharing the vault
		if err != nil {
			return "", "", noop, err
		}
		_ = f.Close()
		candidate := fmt.Sprintf("%s.%s.%04d.%s", TypeIssue, project, ordinal, slug)
		return candidate, filepath.Join(dir, candidate+".md"), func() { _ = os.Remove(marker) }, nil
	}
	if want > 0 {
		// An explicit ordinal is claimed once; there is nothing to retry onto.
		nextFree := "omit --to to take the next free ordinal"
		if held := resolveIssueOrdinalAll(v, project, strconv.Itoa(want)); len(held) > 0 {
			return "", "", noop, fmt.Errorf("ordinal %s.%04d is taken by %s — %s", project, want, strings.Join(held, ", "), nextFree)
		}
		id, path, release, err := claim(want)
		if errors.Is(err, os.ErrExist) {
			return "", "", noop, fmt.Errorf("ordinal %s.%04d is reserved by an in-flight create — %s", project, want, nextFree)
		}
		if err != nil {
			return "", "", noop, fmt.Errorf("reserving ordinal %s.%04d: %w", project, want, err)
		}
		return id, path, release, nil
	}
	for attempt := 0; attempt < 20; attempt++ {
		ordinal, err := nextIssueOrdinal(v, project)
		if err != nil {
			return "", "", noop, err
		}
		id, path, release, err := claim(ordinal)
		if err == nil {
			return id, path, release, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", noop, fmt.Errorf("reserving ordinal %s.%04d: %w", project, ordinal, err)
		}
		// EEXIST: a concurrent create holds this ordinal; retry with the next.
	}
	return "", "", noop, fmt.Errorf("unable to allocate numbered issue ID after 20 attempts")
}

// findIssueBySlug returns the canonical ID and path of an existing
// [issue.]<project>.NNNN.<slug>.md whose slug matches exactly, scoped to
// project. Returns ("", "", false) when none exists. Slug is unique per
// project under the numbered scheme, so at most one file matches.
func findIssueBySlug(v *Vault, project, slug string) (id, path string, found bool) {
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", false
	}
	prefix := project + "."
	suffix := "." + slug + ".md"
	for _, e := range entries {
		name := BareID(TypeIssue, e.Name())
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		ordinal := name[len(prefix) : len(name)-len(suffix)]
		if ordinal != "" && ordinalOnlyRe.MatchString(ordinal) {
			return CanonicalID(TypeIssue, strings.TrimSuffix(name, ".md")), filepath.Join(dir, e.Name()), true
		}
	}
	return "", "", false
}
